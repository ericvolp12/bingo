// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.20.0

package store_queries

import (
	"database/sql"
	"time"
)

type Entry struct {
	Did             string       `json:"did"`
	Handle          string       `json:"handle"`
	IsValid         bool         `json:"is_valid"`
	LastCheckedTime sql.NullTime `json:"last_checked_time"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       sql.NullTime `json:"updated_at"`
}
