package plc

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ericvolp12/bingo/pkg/store"
	"github.com/ericvolp12/bingo/pkg/store/store_queries"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
)

var plcDirectoryRequestHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "plc_directory_request_duration_seconds",
	Help: "Histogram of the time (in seconds) each request to the PLC directory takes",
}, []string{"status_code"})

type Directory struct {
	Endpoint       string
	PLCRateLimiter *rate.Limiter
	PDSRateLimiter *rate.Limiter
	CheckPeriod    time.Duration
	AfterCursor    time.Time
	Logger         *zap.SugaredLogger

	ValidationTTL time.Duration

	RedisClient *redis.Client
	RedisPrefix string

	Store *store.Store
}

type DirectoryEntry struct {
	Did string `json:"did"`
	AKA string `json:"handle"`
}

type RawDirectoryEntry struct {
	JSON json.RawMessage
}

type DirectoryJSONLRow struct {
	Did       string    `json:"did"`
	Operation Operation `json:"operation"`
	Cid       string    `json:"cid"`
	Nullified bool      `json:"nullified"`
	CreatedAt time.Time `json:"createdAt"`
}

type Operation struct {
	AlsoKnownAs []string `json:"alsoKnownAs"`
	Type        string   `json:"type"`
}

var tracer = otel.Tracer("plc-directory")

func NewDirectory(endpoint string, redisClient *redis.Client, store *store.Store, redisPrefix string) (*Directory, error) {
	ctx := context.Background()
	rawLogger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %+v", err)
	}
	logger := rawLogger.Sugar().With("source", "plc_directory")

	cmd := redisClient.Get(ctx, redisPrefix+":last_cursor")
	if cmd.Err() != nil {
		logger.Info("no last cursor found, starting from beginning")
	}

	var lastCursor time.Time
	if cmd.Val() != "" {
		lastCursor, err = time.Parse(time.RFC3339Nano, cmd.Val())
		if err != nil {
			logger.Info("failed to parse last cursor, starting from beginning")
		}
	}

	return &Directory{
		Endpoint:       endpoint,
		Logger:         logger,
		PLCRateLimiter: rate.NewLimiter(rate.Limit(2), 1),
		PDSRateLimiter: rate.NewLimiter(rate.Limit(10), 1),
		CheckPeriod:    30 * time.Second,
		AfterCursor:    lastCursor,

		ValidationTTL: 12 * time.Hour,

		RedisClient: redisClient,
		RedisPrefix: redisPrefix,

		Store: store,
	}, nil
}

func (d *Directory) Start() {
	ticker := time.NewTicker(d.CheckPeriod)
	ctx := context.Background()
	go func() {
		d.fetchDirectoryEntries(ctx)

		for range ticker.C {
			d.fetchDirectoryEntries(ctx)
		}
	}()

	go func() {
		d.ValidateHandles(ctx, 1200, 5*time.Second)
	}()
}

func (d *Directory) fetchDirectoryEntries(ctx context.Context) {
	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	d.Logger.Info("fetching directory entries...")

	for {
		d.Logger.Infof("querying for entries after %s", d.AfterCursor.Format(time.RFC3339Nano))
		req, err := http.NewRequestWithContext(ctx, "GET", d.Endpoint, nil)
		if err != nil {
			d.Logger.Errorf("failed to create request: %+v", err)
			break
		}
		q := req.URL.Query()
		if !d.AfterCursor.IsZero() {
			q.Add("after", d.AfterCursor.Format(time.RFC3339Nano))
		}
		req.URL.RawQuery = q.Encode()
		d.PLCRateLimiter.Wait(ctx)
		start := time.Now()
		resp, err := client.Do(req)
		plcDirectoryRequestHistogram.WithLabelValues(fmt.Sprintf("%d", resp.StatusCode)).Observe(time.Since(start).Seconds())
		if err != nil {
			d.Logger.Errorf("failed to fetch directory entries: %+v", err)
			resp.Body.Close()
			break
		}

		// Create a bufio scanner to read the response line by line
		scanner := bufio.NewScanner(resp.Body)

		var newEntries []DirectoryJSONLRow
		for scanner.Scan() {
			line := scanner.Text()
			var entry DirectoryJSONLRow

			// Try to unmarshal the line into a DirectoryJSONLRow
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				d.Logger.Errorf("failed to unmarshal directory entry: %+v", err)
				resp.Body.Close()
				return
			}

			newEntries = append(newEntries, entry)
		}

		// Check if the scan finished without errors
		if err := scanner.Err(); err != nil {
			d.Logger.Errorf("failed to read response body: %+v", err)
			resp.Body.Close()
			return
		}

		if len(newEntries) <= 1 {
			resp.Body.Close()
			break
		}

		resp.Body.Close()

		for _, entry := range newEntries {
			if len(entry.Operation.AlsoKnownAs) > 0 {
				handle := strings.TrimPrefix(entry.Operation.AlsoKnownAs[0], "at://")
				if handle != "" {
					err := d.Store.Update(ctx, &store.Entry{
						Did:     entry.Did,
						Handle:  handle,
						IsValid: false,
					})
					if err != nil {
						d.Logger.Errorf("failed to update entry: %+v", err)
					}
				}
			}
		}

		d.AfterCursor = newEntries[len(newEntries)-1].CreatedAt
		cmd := d.RedisClient.Set(ctx, d.RedisPrefix+":last_cursor", d.AfterCursor.Format(time.RFC3339Nano), 0)
		if cmd.Err() != nil {
			d.Logger.Errorf("failed to set last cursor: %+v", cmd.Err())
		}
		d.Logger.Infof("fetched %d new directory entries", len(newEntries))
	}

	d.Logger.Info("finished fetching directory entries")
}

var plcDirectoryValidationHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "plc_directory_validation_duration_seconds",
	Help: "Histogram of the time (in seconds) each validation of the PLC directory takes",
}, []string{"is_valid"})

func (d *Directory) ValidateHandles(ctx context.Context, pageSize int, timeBetweenLoops time.Duration) {
	logger := d.Logger.With("source", "plc_directory_validation")
	for {
		select {
		case <-ctx.Done():
			logger.Info("context cancelled, stopping validation loop")
			return
		default:
			if !d.ValidateHandlePage(ctx, pageSize) {
				time.Sleep(timeBetweenLoops)
			}
		}
	}
}

func (d *Directory) ValidateHandlePage(ctx context.Context, pageSize int) bool {
	ctx, span := tracer.Start(ctx, "ValidateHandles")
	defer span.End()

	logger := d.Logger.With("source", "validate_handle_page")

	logger.Info("validating handles entries...")

	start := time.Now()

	entries, err := d.Store.Queries.GetEntriesForValidation(ctx, store_queries.GetEntriesForValidationParams{
		LastCheckedTime: sql.NullTime{Time: time.Now().Add(-d.ValidationTTL), Valid: true},
		Limit:           int32(pageSize),
	})
	if err != nil {
		logger.Errorf("failed to get entries for validation: %+v", err)
		return false
	}

	queryDone := time.Now()

	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	// Validate in 20 goroutines
	sem := semaphore.NewWeighted(20)
	wg := &sync.WaitGroup{}

	storeEntries := []*store.Entry{}
	lk := sync.Mutex{}

	numValid := atomic.Int64{}
	numInvalid := atomic.Int64{}

	for _, entry := range entries {
		wg.Add(1)
		go func(entry store_queries.Entry) {
			defer wg.Done()
			defer sem.Release(1)
			validStart := time.Now()
			valid, errs := d.ValidateHandle(ctx, client, entry.Did, entry.Handle)
			plcDirectoryValidationHistogram.WithLabelValues(fmt.Sprintf("%t", valid)).Observe(time.Since(validStart).Seconds())
			if len(errs) != 0 {
				logger.Errorw("failed to validate handle",
					"did", entry.Did,
					"handle", entry.Handle,
					"errors", errs,
				)
			}
			if valid {
				numValid.Add(1)
			} else {
				numInvalid.Add(1)
			}

			lk.Lock()
			storeEntries = append(storeEntries, &store.Entry{
				Did:             entry.Did,
				Handle:          entry.Handle,
				IsValid:         valid,
				LastCheckedTime: uint64(time.Now().UnixNano()),
			})
			lk.Unlock()
		}(entry)
		sem.Acquire(ctx, 1)
	}

	wg.Wait()

	validationDone := time.Now()

	// Update the entries in the database
	err = d.Store.BulkUpdateEntryValidation(ctx, storeEntries)
	if err != nil {
		logger.Errorf("failed to update entries: %+v", err)
	}

	updateDone := time.Now()

	logger.Infow("finished validating directory entries",
		"valid", numValid.Load(),
		"invalid", numInvalid.Load(),
		"query_time", queryDone.Sub(start).Seconds(),
		"validation_time", validationDone.Sub(queryDone).Seconds(),
		"update_time", updateDone.Sub(validationDone).Seconds(),
		"total_time", updateDone.Sub(start).Seconds(),
	)

	if len(entries) >= pageSize {
		return true
	}

	return false
}

func (d *Directory) ValidateHandle(ctx context.Context, client *http.Client, did string, handle string) (bool, []error) {
	ctx, span := tracer.Start(ctx, "ValidateHandle")
	defer span.End()

	var errs []error
	expectedTxtValue := fmt.Sprintf("did=%s", did)

	// Start by looking up TXT records for the Handle
	txtrecords, err := net.DefaultResolver.LookupTXT(ctx, fmt.Sprintf("_atproto.%s", handle))
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to lookup TXT records for handle: %+v", err))
	} else {
		for _, txt := range txtrecords {
			if txt == expectedTxtValue {
				span.SetAttributes(attribute.Bool("txt_validated", true))
				return true, nil
			}
		}
	}

	// If no TXT records were found, check /.well-known/atproto-did
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s/.well-known/atproto-did", handle), nil)
	if err != nil {
		return false, append(errs, fmt.Errorf("failed to create request for HTTPS validation: %+v", err))
	}

	// If the handle ends in `.bsky.social`, use the PDS rate limiter
	if strings.HasSuffix(handle, ".bsky.social") {
		d.PDSRateLimiter.Wait(ctx)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, append(errs, fmt.Errorf("failed to fetch /.well-known/atproto-did: %+v", err))
	}

	if resp.StatusCode != http.StatusOK {
		span.SetAttributes(attribute.Bool("both_invalid", true))
		span.SetAttributes(attribute.Int("https_status_code", resp.StatusCode))
		return false, append(errs, fmt.Errorf("failed to fetch /.well-known/atproto-did: %s", resp.Status))
	}

	// There should only be one line in the response with the contenr of the DID
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == did {
			span.SetAttributes(attribute.Bool("https_validated", true))
			return true, nil
		}
	}

	span.SetAttributes(attribute.Bool("both_invalid", true))
	return false, append(errs, fmt.Errorf("failed to find DID in /.well-known/atproto-did"))
}
