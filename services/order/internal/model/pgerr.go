package model

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres SQLSTATEs this service distinguishes.
const (
	sqlStateUniqueViolation = "23505"
	// 55P03 covers both an explicit FOR UPDATE NOWAIT rejection and a
	// lock_timeout expiry while waiting for a row lock (docs/TZ.md §7.4
	// sets lock_timeout to 1s).
	sqlStateLockNotAvailable = "55P03"
	// 57014 is statement_timeout expiry — a different failure, kept
	// separate so a slow query is never mistaken for a contended row.
	sqlStateQueryCanceled = "57014"
)

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// IsUniqueViolation reports whether err is a duplicate-key error.
func IsUniqueViolation(err error) bool {
	return pgCode(err) == sqlStateUniqueViolation
}

// IsLockUnavailable reports whether err means "another transaction holds
// this row" — the signal behind §7.4's REQUEST_IN_PROGRESS.
func IsLockUnavailable(err error) bool {
	return pgCode(err) == sqlStateLockNotAvailable
}

// IsQueryCanceled reports a statement_timeout expiry.
func IsQueryCanceled(err error) bool {
	return pgCode(err) == sqlStateQueryCanceled
}
