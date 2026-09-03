package model

import (
	"fmt"
	"strings"
)

// stringSlice scans a Postgres text[] column into a Go []string.
//
// pgx's stdlib database/sql driver returns an array column's raw text
// representation ("{a,b,c}") as a plain string rather than decoding it
// into a Go slice — database/sql's generic Scan has no built-in
// conversion from string to []string, so without this it fails with
// "unsupported Scan ... storing driver.Value type string into type
// *[]string" (verified against a real Postgres instance before writing
// this). Splitting naively on "," after trimming the braces is safe here
// specifically because every value ever stored in this column is a
// simple BCP-47 locale tag (venues.locales, venues.default_locale) —
// this is not a general-purpose Postgres array parser and would need
// proper quote/escape handling for arbitrary strings.
type stringSlice []string

func (s *stringSlice) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}
	str, ok := src.(string)
	if !ok {
		return fmt.Errorf("model: cannot scan %T into stringSlice", src)
	}
	str = strings.TrimSuffix(strings.TrimPrefix(str, "{"), "}")
	if str == "" {
		*s = []string{}
		return nil
	}
	*s = strings.Split(str, ",")
	return nil
}

// pgTextArrayLiteral renders elems as a Postgres array literal ("{a,b}")
// for use with an explicit ::text[] cast in a query parameter — the
// write-side counterpart to stringSlice, and subject to the same
// simple-values-only caveat.
func pgTextArrayLiteral(elems []string) string {
	return "{" + strings.Join(elems, ",") + "}"
}
