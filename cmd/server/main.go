package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/ericvolp12/bingo/gen/bingo/v1/bingov1connect"
	"github.com/ericvolp12/bingo/pkg/lookup"
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

	redisClient := redis.NewClient(&redis.Options{
		Addr: cctx.String("redis-address"),
	})

	// Ping the redis server to make sure it's up.
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}

	st, err := store.NewStore(ctx, redisClient, cctx.String("redis-prefix"), cctx.String("postgres-url"))
	if err != nil {
		return err
	}

	st.Update(ctx, &store.Entry{
		Handle:          "jaz.bsky.social",
		Did:             "did:plc:q6gjnaw2blty4crticxkmujt",
		IsValid:         true,
		LastCheckedTime: uint64(time.Now().UnixNano()),
	})

	lookupServer := lookup.NewServer(st)

	mux := http.NewServeMux()

	interceptor := connect_go_prometheus.NewInterceptor()

	path, handler := bingov1connect.NewBingoServiceHandler(lookupServer, connect.WithInterceptors(interceptor))
	mux.Handle(path, handler)

	mux.Handle("/metrics", promhttp.Handler())

	listenAddr := fmt.Sprintf(":%d", cctx.Int("port"))
	log.Printf("listening on %s", listenAddr)

	http.ListenAndServe(
		listenAddr,
		// Use h2c so we can serve HTTP/2 without TLS.
		h2c.NewHandler(mux, &http2.Server{}),
	)

	return nil
}
