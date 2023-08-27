package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/ericvolp12/bingo/gen/bingo/v1/bingov1connect"
	"github.com/ericvolp12/bingo/pkg/lookup"
	"github.com/ericvolp12/bingo/pkg/plc"
	"github.com/ericvolp12/bingo/pkg/store"
	connect_go_prometheus "github.com/ericvolp12/connect-go-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.App{
		Name:    "bingo",
		Usage:   "a DID lookup service",
		Version: "0.0.1",
	}

	app.Flags = []cli.Flag{
		&cli.IntFlag{
			Name:    "port",
			Usage:   "port to listen on",
			Value:   8080,
			EnvVars: []string{"PORT"},
		},
		&cli.StringFlag{
			Name:    "redis-address",
			Usage:   "redis address for storing entries",
			Value:   "localhost:6379",
			EnvVars: []string{"REDIS_ADDRESS"},
		},
		&cli.StringFlag{
			Name:    "redis-prefix",
			Usage:   "redis prefix for storing entries",
			Value:   "bingo",
			EnvVars: []string{"REDIS_PREFIX"},
		},
		&cli.StringFlag{
			Name:    "postgres-url",
			Usage:   "postgres connection string",
			Value:   "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
			EnvVars: []string{"POSTGRES_URL"},
		},
		&cli.StringFlag{
			Name:    "plc-endpoint",
			Usage:   "plc endpoint",
			Value:   "https://plc.directory/export",
			EnvVars: []string{"PLC_ENDPOINT"},
		},
	}

	app.Action = Bingo

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

var tracer = otel.Tracer("bingo")

// Bingo is the main function for the lookup service
func Bingo(cctx *cli.Context) error {
	ctx := cctx.Context

	rawlog, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to create logger: %+v\n", err)
	}
	defer func() {
		log.Printf("main function teardown\n")
		err := rawlog.Sync()
		if err != nil {
			log.Printf("failed to sync logger on teardown: %+v", err.Error())
		}
	}()

	log := rawlog.Sugar().With("source", "bingo_main")

	log.Info("starting up")

	redisClient := redis.NewClient(&redis.Options{
		Addr: cctx.String("redis-address"),
	})

	// Ping the redis server to make sure it's up.
	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}

	st, err := store.NewStore(ctx, redisClient, cctx.String("redis-prefix"), cctx.String("postgres-url"))
	if err != nil {
		return err
	}

	plc, err := plc.NewDirectory(cctx.String("plc-endpoint"), redisClient, st, cctx.String("redis-prefix"))
	if err != nil {
		return err
	}

	plc.Start()

	lookupServer := lookup.NewServer(st)

	mux := http.NewServeMux()

	interceptor := connect_go_prometheus.NewInterceptor()

	path, handler := bingov1connect.NewBingoServiceHandler(lookupServer, connect.WithInterceptors(interceptor))
	mux.Handle(path, handler)

	mux.Handle("/metrics", promhttp.Handler())

	listenAddr := fmt.Sprintf(":%d", cctx.Int("port"))

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	serverShutdown := make(chan struct{})
	go func() {
		defer close(serverShutdown)
		log := log.With("source", "http_server")
		log.Infof("listening on %s", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server shutdown: %+v\n", err)
		}
	}()

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-signals:
		log.Info("shutting down on signal")
	case <-ctx.Done():
		log.Info("shutting down on context done")

	}

	log.Info("shutting down, waiting for workers to clean up...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("failed to shutdown http server: %+v\n", err)
	}

	select {
	case <-serverShutdown:
		log.Info("http server shut down successfully")
	case <-ctx.Done():
		log.Info("http server shutdown timed out")
	}

	log.Info("shut down successfully")

	return nil
}
