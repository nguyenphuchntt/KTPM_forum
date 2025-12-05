package retry

import (
	"strings"
)

func IsNonRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errorMessage := strings.ToLower(err.Error())

	nonRetryablePatterns := []string{
		"sql: no rows in result set",           // Normal query result, should not retry
		"duplicate entry",                      // MySQL Error 1062
		"duplicate key",                        // Constraint violation
		"foreign key constraint",               // MySQL Error 1452
		"cannot add or update",                 // Constraint violation
		"data too long",                        // MySQL Error 1406
		"incorrect string value",               // Data validation error
		"access denied",                        // Permission error
		"unknown database",                     // Configuration error
		"unknown table",                        // Schema error
		"unknown column",                       // Schema error
		"syntax error",                         // SQL syntax error
		"you have an error in your sql syntax", // SQL syntax error
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errorMessage, pattern) {
			return true
		}
	}
	
	return false
}