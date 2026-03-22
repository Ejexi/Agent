package sqlite

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/SecDuckOps/shared/core/session"
	"github.com/SecDuckOps/shared/types"
)

// translateErr converts any database error into a types.AppError.
// Raw sql or driver errors NEVER leak past this function.
func translateErr(err error, context string) error {
	if err == nil {
		return nil
	}

	// 1. No rows -> domain ErrNotFound
	if errors.Is(err, sql.ErrNoRows) {
		return types.Wrap(session.ErrNotFound, types.ErrCodeNotFound, context)
	}

	// 2. UNIQUE / PRIMARY KEY constraint -> domain ErrAlreadyExists
	msg := err.Error()
	if strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "PRIMARY KEY constraint failed") {
		return types.Wrap(session.ErrAlreadyExists, types.ErrCodeAlreadyExists, context)
	}

	// 3. Foreign key constraint -> domain ErrNotFound (parent doesn't exist)
	if strings.Contains(msg, "FOREIGN KEY constraint failed") {
		return types.Wrap(session.ErrNotFound, types.ErrCodeNotFound, context+": referenced record does not exist")
	}

	// 4. Anything else is an internal/unexpected error
	return types.Wrap(err, types.ErrCodeInternal, context)
}

// checkRowsAffected verifies that an UPDATE/DELETE touched exactly one row.
// Accepts *types.AppError directly for compile-time safety.
func checkRowsAffected(result sql.Result, sentinel *types.AppError, context string) error {
	n, err := result.RowsAffected()
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, context+": reading affected rows")
	}
	if n == 0 {
		return types.Wrap(sentinel, sentinel.Code, context)
	}
	return nil
}
