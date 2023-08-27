package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/ericvolp12/bingo/pkg/store/store_queries"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

type Store struct {
	RedisPrefix string
	Redis       *redis.Client
	DB          *sql.DB
	Queries     *store_queries.Queries
}

type Entry struct {
	Handle          string `json:"handle"`
	Did             string `json:"did"`
	IsValid         bool   `json:"valid"`
	LastCheckedTime uint64 `json:"checked"`
}

var ErrNotFound = errors.New("bingo: not found")

var tracer = otel.Tracer("bingo/store")

var byDidPrefix = "d"
var byHandlePrefix = "h"

func NewStore(
	ctx context.Context,
	client *redis.Client,
	prefix string,
	postgresConnect string,
) (*Store, error) {
	ctx, span := tracer.Start(ctx, "NewStore")
	defer span.End()

	var db *sql.DB
	var err error

	for i := 0; i < 5; i++ {
		db, err = otelsql.Open(
			"postgres",
			postgresConnect,
			otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
		)
		if err != nil {
			return nil, err
		}

		db.SetMaxOpenConns(50)

		err = otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(
			semconv.DBSystemPostgreSQL,
		))
		if err != nil {
			return nil, err
		}

		err = db.Ping()
		if err == nil {
			break
		}

		db.Close() // Close the connection if it failed.
		time.Sleep(5 * time.Second)
	}

	if err != nil {
		return nil, err
	}

	queries := store_queries.New(db)

	// Iterate over all entries in postgres and set them in redis
	pageSize := 1000
	offset := 0
	for {
		dbEntries, err := queries.GetEntries(ctx, store_queries.GetEntriesParams{
			Limit:  int32(pageSize),
			Offset: int32(offset),
		})
		if err != nil {
			return nil, fmt.Errorf("bingo: failed to list entries: %w", err)
		}

		if len(dbEntries) == 0 {
			break
		}

		pipeline := client.Pipeline()

		for _, dbEntry := range dbEntries {
			entry := &Entry{
				Handle:          dbEntry.Handle,
				Did:             dbEntry.Did,
				IsValid:         dbEntry.IsValid,
				LastCheckedTime: uint64(dbEntry.LastCheckedTime.Time.UnixNano()),
			}

			byDidKey := fmt.Sprintf("%s_%s_%s", prefix, byDidPrefix, entry.Did)
			byHandleKey := fmt.Sprintf("%s_%s_%s", prefix, byHandlePrefix, entry.Handle)

			val, err := json.Marshal(entry)
			if err != nil {
				return nil, fmt.Errorf("bingo: failed to marshal entry: %w", err)
			}

			pipeline.Set(ctx, byDidKey, val, 0)
			pipeline.Set(ctx, byHandleKey, val, 0)
		}

		_, err = pipeline.Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("bingo: failed to execute pipeline: %w", err)
		}

		offset += pageSize
	}

	return &Store{
		RedisPrefix: prefix,
		Redis:       client,
		DB:          db,
		Queries:     queries,
	}, nil
}

func (s *Store) Lookup(ctx context.Context, handleOrDid string) (*Entry, error) {
	ctx, span := tracer.Start(ctx, "Lookup")
	defer span.End()
	span.SetAttributes(attribute.String("handleOrDid", handleOrDid))

	var val string
	var err error

	if IsDID(handleOrDid) {
		key := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byDidPrefix, handleOrDid)
		val, err = s.Redis.Get(ctx, key).Result()
	} else {
		key := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byHandlePrefix, handleOrDid)
		val, err = s.Redis.Get(ctx, key).Result()
	}

	if err != nil {
		if err == redis.Nil {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("bingo: failed to lookup entry: %w", err)
	}

	entry := &Entry{}
	if err := json.Unmarshal([]byte(val), entry); err != nil {
		return nil, fmt.Errorf("bingo: failed to unmarshal entry: %w", err)
	}

	return entry, nil
}

func (s *Store) BulkLookupByDid(ctx context.Context, dids []string) ([]*Entry, error) {
	ctx, span := tracer.Start(ctx, "BulkLookupByDid")
	defer span.End()

	entries := []*Entry{}

	pipeline := s.Redis.Pipeline()

	for _, did := range dids {
		key := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byDidPrefix, did)
		pipeline.Get(ctx, key)
	}

	results, err := pipeline.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("bingo: failed to execute pipeline: %w", err)
	}

	for _, result := range results {
		val, err := result.(*redis.StringCmd).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, fmt.Errorf("bingo: failed to lookup entry: %w", err)
		}

		entry := &Entry{}
		if err := json.Unmarshal([]byte(val), entry); err != nil {
			return nil, fmt.Errorf("bingo: failed to unmarshal entry: %w", err)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func (s *Store) BulkLookupByHandle(ctx context.Context, handles []string) ([]*Entry, error) {
	ctx, span := tracer.Start(ctx, "BulkLookupByHandle")
	defer span.End()

	entries := []*Entry{}

	pipeline := s.Redis.Pipeline()

	for _, handle := range handles {
		key := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byHandlePrefix, handle)
		pipeline.Get(ctx, key)
	}

	results, err := pipeline.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("bingo: failed to execute pipeline: %w", err)
	}

	for _, result := range results {
		val, err := result.(*redis.StringCmd).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			return nil, fmt.Errorf("bingo: failed to lookup entry: %w", err)
		}

		entry := &Entry{}
		if err := json.Unmarshal([]byte(val), entry); err != nil {
			return nil, fmt.Errorf("bingo: failed to unmarshal entry: %w", err)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func (s *Store) Update(ctx context.Context, entry *Entry) error {
	ctx, span := tracer.Start(ctx, "Update")
	defer span.End()

	lastCheckedSQLTime := sql.NullTime{}
	if entry.LastCheckedTime != 0 {
		lastCheckedSQLTime = sql.NullTime{
			Time:  time.Unix(0, int64(entry.LastCheckedTime)),
			Valid: true,
		}
	}

	// Update the entry in postgres
	err := s.Queries.UpdateEntry(ctx, store_queries.UpdateEntryParams{
		Handle:          entry.Handle,
		Did:             entry.Did,
		IsValid:         entry.IsValid,
		LastCheckedTime: lastCheckedSQLTime,
	})
	if err != nil {
		return fmt.Errorf("bingo: failed to update entry: %w", err)
	}

	// Set the entry in redis
	byDidKey := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byDidPrefix, entry.Did)
	byHandleKey := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byHandlePrefix, entry.Handle)

	// Lookup the old entry by did
	byDidVal, err := s.Redis.Get(ctx, byDidKey).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("bingo: failed to lookup entry by did: %w", err)
	}

	if byDidVal != "" {
		// Unpack the entry into an Entry
		oldDidEntry := &Entry{}
		if err := json.Unmarshal([]byte(byDidVal), oldDidEntry); err != nil {
			return fmt.Errorf("bingo: failed to unmarshal old entry: %w", err)
		}

		// If the old entry's handle is different from the new entry's handle, delete the old entry by handle
		if entry.Handle != oldDidEntry.Handle && entry.Did == oldDidEntry.Did {
			byOldHandleKey := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byHandlePrefix, oldDidEntry.Handle)
			err := s.Redis.Del(ctx, byOldHandleKey).Err()
			if err != nil {
				return fmt.Errorf("bingo: failed to delete old entry by handle: %w", err)
			}
		}
	}

	// Set both entries
	val, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("bingo: failed to marshal entry: %w", err)
	}

	err = s.Redis.Set(ctx, byDidKey, val, 0).Err()
	if err != nil {
		return fmt.Errorf("bingo: failed to set entry by did: %w", err)
	}

	err = s.Redis.Set(ctx, byHandleKey, val, 0).Err()
	if err != nil {
		return fmt.Errorf("bingo: failed to set entry by handle: %w", err)
	}

	return nil
}

func (s *Store) BulkUpdateEntryValidation(ctx context.Context, entries []*Entry) error {
	ctx, span := tracer.Start(ctx, "BulkUpdateEntries")
	defer span.End()

	// Split valid and invalid entries
	validDids := []string{}
	invalidDids := []string{}

	for _, entry := range entries {
		if entry.IsValid {
			validDids = append(validDids, entry.Did)
		} else {
			invalidDids = append(invalidDids, entry.Did)
		}
	}

	lastCheckedSQLTime := sql.NullTime{
		Time:  time.Now(),
		Valid: true,
	}

	// Update entries in two batches, one for valid entries and one for invalid entries
	if len(validDids) > 0 {
		err := s.Queries.UpdateEntriesValidation(ctx, store_queries.UpdateEntriesValidationParams{
			LastCheckedTime: lastCheckedSQLTime,
			IsValid:         true,
			Dids:            validDids,
		})
		if err != nil {
			return fmt.Errorf("bingo: failed to update entries: %w", err)
		}
	}
	if len(invalidDids) > 0 {
		err := s.Queries.UpdateEntriesValidation(ctx, store_queries.UpdateEntriesValidationParams{
			LastCheckedTime: lastCheckedSQLTime,
			IsValid:         false,
			Dids:            invalidDids,
		})
		if err != nil {
			return fmt.Errorf("bingo: failed to update entries: %w", err)
		}
	}

	// Set the entries in redis
	pipeline := s.Redis.Pipeline()

	for _, entry := range entries {
		byDidKey := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byDidPrefix, entry.Did)
		byHandleKey := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byHandlePrefix, entry.Handle)

		// Set both entries
		val, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("bingo: failed to marshal entry: %w", err)
		}

		pipeline.Set(ctx, byDidKey, val, 0)
		pipeline.Set(ctx, byHandleKey, val, 0)
	}

	_, err := pipeline.Exec(ctx)
	if err != nil {
		return fmt.Errorf("bingo: failed to execute pipeline: %w", err)
	}

	return nil
}

func (s *Store) Delete(ctx context.Context, did string) error {
	ctx, span := tracer.Start(ctx, "Delete")
	defer span.End()
	span.SetAttributes(attribute.String("did", did))

	// Lookup the old entry by did
	byDidKey := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byDidPrefix, did)
	byDidVal, err := s.Redis.Get(ctx, byDidKey).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("bingo: failed to lookup entry by did: %w", err)
	}

	if byDidVal == "" {
		return nil
	}

	// Unpack the entry into an Entry
	oldDidEntry := &Entry{}
	if err := json.Unmarshal([]byte(byDidVal), oldDidEntry); err != nil {
		return fmt.Errorf("bingo: failed to unmarshal old entry: %w", err)
	}

	// Delete the entry by did
	err = s.Redis.Del(ctx, byDidKey).Err()
	if err != nil {
		return fmt.Errorf("bingo: failed to delete entry by did: %w", err)
	}

	// Delete the entry by handle
	byHandleKey := fmt.Sprintf("%s_%s_%s", s.RedisPrefix, byHandlePrefix, oldDidEntry.Handle)
	err = s.Redis.Del(ctx, byHandleKey).Err()
	if err != nil {
		return fmt.Errorf("bingo: failed to delete entry by handle: %w", err)
	}

	return nil
}

func IsDID(handleOrDid string) bool {
	return strings.HasPrefix(handleOrDid, "did:")
}
