package retry

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

func IsNonRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check PostgreSQL SQLSTATE codes (more reliable than string matching).
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return true
		case "23503": // foreign_key_violation
			return true
		case "22001": // string_data_right_truncation
			return true
		case "42000": // syntax_error
			return true
		case "42601": // syntax_error
			return true
		case "42P01": // undefined_table
			return true
		case "42703": // undefined_column
			return true
		}
	}

	errorMessage := strings.ToLower(err.Error())

	// Legacy string patterns kept as a fallback for errors that arrive wrapped
	// or already stringified (SQLSTATE checks above take precedence).
	nonRetryablePatterns := []string{
		"sql: no rows in result set", // Normal query result, should not retry
		"duplicate key",              // Constraint violation
		"duplicate entry",            // Legacy MySQL Error 1062
		"foreign key constraint",     // FK violation
		"cannot add or update",       // Constraint violation
		"data too long",              // Legacy MySQL Error 1406
		"value too long",             // PostgreSQL 22001 text form
		"incorrect string value",     // Data validation error
		"access denied",              // Permission error
		"unknown database",           // Configuration error
		"does not exist",             // PostgreSQL undefined table/column
		"unknown table",              // Legacy schema error
		"unknown column",             // Legacy schema error
		"syntax error",               // SQL syntax error
		"you have an error in your sql syntax", // Legacy MySQL syntax error
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errorMessage, pattern) {
			return true
		}
	}
	
	return false
}