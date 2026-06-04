// Package store is the repository layer. All DB access lives here as hand-written
// SQL executed through pgx. Dynamic WHERE clauses are assembled with positional
// placeholders ($1, $2, ...) and an args slice; values are NEVER concatenated
// into the SQL string.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrNotFound is returned by single-row lookups when no row matches.
var ErrNotFound = errors.New("store: not found")

// ErrStatusRequiresUser is returned by Questions.Query when a status filter
// (unanswered/wrong/favorite) is requested without a UserID. Handlers translate
// this into a 400 VALIDATION response.
var ErrStatusRequiresUser = errors.New("store: status filter requires a user")

// Querier is the subset of pgxpool.Pool / pgx.Tx shared by the stores, so the
// same query methods can run against a pool or inside a transaction.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}
