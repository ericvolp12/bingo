package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type Store struct {
	RedisPrefix string
	Redis       *redis.Client
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

func NewStore(ctx context.Context, client *redis.Client, prefix string) (*Store, error) {
	ctx, span := tracer.Start(ctx, "NewStore")
	defer span.End()

	return &Store{
		RedisPrefix: prefix,
		Redis:       client,
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
	if err != nil {
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
	if err != nil {
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
