// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.20.0

package store_queries

import (
	"context"
	"database/sql"
	"fmt"
)

type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

func Prepare(ctx context.Context, db DBTX) (*Queries, error) {
	q := Queries{db: db}
	var err error
	if q.getEntriesStmt, err = db.PrepareContext(ctx, getEntries); err != nil {
		return nil, fmt.Errorf("error preparing query GetEntries: %w", err)
	}
	if q.getEntriesForValidationStmt, err = db.PrepareContext(ctx, getEntriesForValidation); err != nil {
		return nil, fmt.Errorf("error preparing query GetEntriesForValidation: %w", err)
	}
	if q.getEntryByDIDStmt, err = db.PrepareContext(ctx, getEntryByDID); err != nil {
		return nil, fmt.Errorf("error preparing query GetEntryByDID: %w", err)
	}
	if q.getEntryByHandleStmt, err = db.PrepareContext(ctx, getEntryByHandle); err != nil {
		return nil, fmt.Errorf("error preparing query GetEntryByHandle: %w", err)
	}
	if q.updateEntriesValidationStmt, err = db.PrepareContext(ctx, updateEntriesValidation); err != nil {
		return nil, fmt.Errorf("error preparing query UpdateEntriesValidation: %w", err)
	}
	if q.updateEntryStmt, err = db.PrepareContext(ctx, updateEntry); err != nil {
		return nil, fmt.Errorf("error preparing query UpdateEntry: %w", err)
	}
	return &q, nil
}

func (q *Queries) Close() error {
	var err error
	if q.getEntriesStmt != nil {
		if cerr := q.getEntriesStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getEntriesStmt: %w", cerr)
		}
	}
	if q.getEntriesForValidationStmt != nil {
		if cerr := q.getEntriesForValidationStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getEntriesForValidationStmt: %w", cerr)
		}
	}
	if q.getEntryByDIDStmt != nil {
		if cerr := q.getEntryByDIDStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getEntryByDIDStmt: %w", cerr)
		}
	}
	if q.getEntryByHandleStmt != nil {
		if cerr := q.getEntryByHandleStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing getEntryByHandleStmt: %w", cerr)
		}
	}
	if q.updateEntriesValidationStmt != nil {
		if cerr := q.updateEntriesValidationStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing updateEntriesValidationStmt: %w", cerr)
		}
	}
	if q.updateEntryStmt != nil {
		if cerr := q.updateEntryStmt.Close(); cerr != nil {
			err = fmt.Errorf("error closing updateEntryStmt: %w", cerr)
		}
	}
	return err
}

func (q *Queries) exec(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) (sql.Result, error) {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).ExecContext(ctx, args...)
	case stmt != nil:
		return stmt.ExecContext(ctx, args...)
	default:
		return q.db.ExecContext(ctx, query, args...)
	}
}

func (q *Queries) query(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) (*sql.Rows, error) {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).QueryContext(ctx, args...)
	case stmt != nil:
		return stmt.QueryContext(ctx, args...)
	default:
		return q.db.QueryContext(ctx, query, args...)
	}
}

func (q *Queries) queryRow(ctx context.Context, stmt *sql.Stmt, query string, args ...interface{}) *sql.Row {
	switch {
	case stmt != nil && q.tx != nil:
		return q.tx.StmtContext(ctx, stmt).QueryRowContext(ctx, args...)
	case stmt != nil:
		return stmt.QueryRowContext(ctx, args...)
	default:
		return q.db.QueryRowContext(ctx, query, args...)
	}
}

type Queries struct {
	db                          DBTX
	tx                          *sql.Tx
	getEntriesStmt              *sql.Stmt
	getEntriesForValidationStmt *sql.Stmt
	getEntryByDIDStmt           *sql.Stmt
	getEntryByHandleStmt        *sql.Stmt
	updateEntriesValidationStmt *sql.Stmt
	updateEntryStmt             *sql.Stmt
}

func (q *Queries) WithTx(tx *sql.Tx) *Queries {
	return &Queries{
		db:                          tx,
		tx:                          tx,
		getEntriesStmt:              q.getEntriesStmt,
		getEntriesForValidationStmt: q.getEntriesForValidationStmt,
		getEntryByDIDStmt:           q.getEntryByDIDStmt,
		getEntryByHandleStmt:        q.getEntryByHandleStmt,
		updateEntriesValidationStmt: q.updateEntriesValidationStmt,
		updateEntryStmt:             q.updateEntryStmt,
	}
}
