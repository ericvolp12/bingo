-- name: UpdateEntry :exec
INSERT INTO entries (did, handle, is_valid, last_checked_time)
VALUES ($1, $2, $3, $4) ON CONFLICT (did) DO
UPDATE
SET handle = EXCLUDED.handle,
    updated_at = EXCLUDED.updated_at
WHERE entries.did = EXCLUDED.did;
-- name: GetEntryByDID :one
SELECT *
FROM entries
WHERE did = $1;
-- name: GetEntryByHandle :one
SELECT *
FROM entries
WHERE handle = $1;
-- name: GetEntriesForValidation :many
SELECT *
from entries
WHERE last_checked_time is NULL
    OR last_checked_time < $1
ORDER BY did
LIMIT $2;
-- name: UpdateEntriesValidation :exec
UPDATE entries
SET last_checked_time = $1,
    is_valid = $2
WHERE did = ANY(sqlc.arg('dids')::text []);
-- name: GetEntries :many
SELECT *
FROM entries
ORDER BY did
LIMIT $1 OFFSET $2;
