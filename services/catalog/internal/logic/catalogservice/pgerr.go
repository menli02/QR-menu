package catalogservicelogic

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres's standard SQLSTATEs — see
// https://www.postgresql.org/docs/current/errcodes-appendix.html.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
