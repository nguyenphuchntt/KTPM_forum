package database

import (
	"database/sql"
	"time"

	"forum/server/metrics"
)

// QueryWithMetrics wraps db.Query with metrics collection
func QueryWithMetrics(db *sql.DB, queryType, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := db.Query(query, args...)
	duration := time.Since(start).Seconds()
	
	metrics.DbQueryDuration.WithLabelValues(queryType).Observe(duration)
	if err != nil {
		metrics.DbQueryErrors.WithLabelValues(queryType).Inc()
	}
	
	return rows, err
}

// ExecWithMetrics wraps db.Exec with metrics collection
func ExecWithMetrics(db *sql.DB, queryType, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := db.Exec(query, args...)
	duration := time.Since(start).Seconds()
	
	metrics.DbQueryDuration.WithLabelValues(queryType).Observe(duration)
	if err != nil {
		metrics.DbQueryErrors.WithLabelValues(queryType).Inc()
	}
	
	return result, err
}

// QueryRowWithMetrics wraps db.QueryRow with metrics collection
// Note: QueryRow doesn't return error until Scan is called, so we only measure duration here
func QueryRowWithMetrics(db *sql.DB, queryType, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	row := db.QueryRow(query, args...)
	duration := time.Since(start).Seconds()
	
	metrics.DbQueryDuration.WithLabelValues(queryType).Observe(duration)
	
	return row
}

// QueryRowWithMetricsAndError wraps db.QueryRow and records errors when Scan fails
func QueryRowWithMetricsAndError(db *sql.DB, queryType, query string, args ...interface{}) (*sql.Row, func(error)) {
	start := time.Now()
	row := db.QueryRow(query, args...)
	duration := time.Since(start).Seconds()
	
	metrics.DbQueryDuration.WithLabelValues(queryType).Observe(duration)
	
	// Return row and a callback to record errors
	errorCallback := func(scanErr error) {
		if scanErr != nil && scanErr != sql.ErrNoRows {
			metrics.DbQueryErrors.WithLabelValues(queryType).Inc()
		}
	}
	
	return row, errorCallback
}
