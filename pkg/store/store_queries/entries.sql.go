// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.20.0
// source: entries.sql

package store_queries

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
)

const getEntries = `-- name: GetEntries :many
SELECT did, handle, is_valid, last_checked_time, created_at, updated_at
FROM entries
ORDER BY did
LIMIT $1 OFFSET $2
`

type GetEntriesParams struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

func (q *Queries) GetEntries(ctx context.Context, arg GetEntriesParams) ([]Entry, error) {
	rows, err := q.query(ctx, q.getEntriesStmt, getEntries, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Entry
	for rows.Next() {
		var i Entry
		if err := rows.Scan(
			&i.Did,
			&i.Handle,
			&i.IsValid,
			&i.LastCheckedTime,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getEntriesForValidation = `-- name: GetEntriesForValidation :many
SELECT did, handle, is_valid, last_checked_time, created_at, updated_at
from entries
WHERE last_checked_time is NULL
    OR last_checked_time < $1
ORDER BY did
LIMIT $2
`

type GetEntriesForValidationParams struct {
	LastCheckedTime sql.NullTime `json:"last_checked_time"`
	Limit           int32        `json:"limit"`
}

func (q *Queries) GetEntriesForValidation(ctx context.Context, arg GetEntriesForValidationParams) ([]Entry, error) {
	rows, err := q.query(ctx, q.getEntriesForValidationStmt, getEntriesForValidation, arg.LastCheckedTime, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Entry
	for rows.Next() {
		var i Entry
		if err := rows.Scan(
			&i.Did,
			&i.Handle,
			&i.IsValid,
			&i.LastCheckedTime,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getEntryByDID = `-- name: GetEntryByDID :one
SELECT did, handle, is_valid, last_checked_time, created_at, updated_at
FROM entries
WHERE did = $1
`

func (q *Queries) GetEntryByDID(ctx context.Context, did string) (Entry, error) {
	row := q.queryRow(ctx, q.getEntryByDIDStmt, getEntryByDID, did)
	var i Entry
	err := row.Scan(
		&i.Did,
		&i.Handle,
		&i.IsValid,
		&i.LastCheckedTime,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const getEntryByHandle = `-- name: GetEntryByHandle :one
SELECT did, handle, is_valid, last_checked_time, created_at, updated_at
FROM entries
WHERE handle = $1
`

func (q *Queries) GetEntryByHandle(ctx context.Context, handle string) (Entry, error) {
	row := q.queryRow(ctx, q.getEntryByHandleStmt, getEntryByHandle, handle)
	var i Entry
	err := row.Scan(
		&i.Did,
		&i.Handle,
		&i.IsValid,
		&i.LastCheckedTime,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

const updateEntriesValidation = `-- name: UpdateEntriesValidation :exec
UPDATE entries
SET last_checked_time = $1,
    is_valid = $2
WHERE did = ANY($3::text [])
`

type UpdateEntriesValidationParams struct {
	LastCheckedTime sql.NullTime `json:"last_checked_time"`
	IsValid         bool         `json:"is_valid"`
	Dids            []string     `json:"dids"`
}

func (q *Queries) UpdateEntriesValidation(ctx context.Context, arg UpdateEntriesValidationParams) error {
	_, err := q.exec(ctx, q.updateEntriesValidationStmt, updateEntriesValidation, arg.LastCheckedTime, arg.IsValid, pq.Array(arg.Dids))
	return err
}

const updateEntry = `-- name: UpdateEntry :exec
INSERT INTO entries (did, handle, is_valid, last_checked_time)
VALUES ($1, $2, $3, $4) ON CONFLICT (did) DO
UPDATE
SET handle = EXCLUDED.handle,
    updated_at = EXCLUDED.updated_at
WHERE entries.did = EXCLUDED.did
`

type UpdateEntryParams struct {
	Did             string       `json:"did"`
	Handle          string       `json:"handle"`
	IsValid         bool         `json:"is_valid"`
	LastCheckedTime sql.NullTime `json:"last_checked_time"`
}

func (q *Queries) UpdateEntry(ctx context.Context, arg UpdateEntryParams) error {
	_, err := q.exec(ctx, q.updateEntryStmt, updateEntry,
		arg.Did,
		arg.Handle,
		arg.IsValid,
		arg.LastCheckedTime,
	)
	return err
}
