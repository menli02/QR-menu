package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

// ErrNotFound is sqlx.ErrNotFound (== sql.ErrNoRows), re-exported so
// callers outside this package don't need to import sqlx just to compare
// errors.
var ErrNotFound = sqlx.ErrNotFound
