package identityservicelogic

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// isUniqueViolation reports whether err is Postgres's standard
// unique-constraint-violation SQLSTATE (23505) — see
// https://www.postgresql.org/docs/current/errcodes-appendix.html. Used to
// turn a duplicate (venue_id, email) insert into codes.AlreadyExists
// instead of a bare codes.Internal.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
