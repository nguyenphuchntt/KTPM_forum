package database

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// rebind converts the `?` placeholders used throughout the repositories into
// the `$1, $2, ...` form PostgreSQL expects. Keeping the queries written with
// `?` means they stay portable and callers never have to care about the driver.
func rebind(query string) string {
	return sqlx.Rebind(sqlx.DOLLAR, query)
}

// QueryWithMetrics wraps db.Query with rebind
func QueryWithMetrics(db *sql.DB, queryType, query string, args ...interface{}) (*sql.Rows, error) {
	return db.Query(rebind(query), args...)
}

// ExecWithMetrics wraps db.Exec with rebind
func ExecWithMetrics(db *sql.DB, queryType, query string, args ...interface{}) (sql.Result, error) {
	return db.Exec(rebind(query), args...)
}

func ExecWithMetricsTx(tx *sql.Tx, queryType, query string, args ...interface{}) (sql.Result, error) {
	return tx.Exec(rebind(query), args...)
}

// QueryRowWithMetricsTx wraps tx.QueryRow with rebind
func QueryRowWithMetricsTx(tx *sql.Tx, queryType, query string, args ...interface{}) *sql.Row {
	return tx.QueryRow(rebind(query), args...)
}

// QueryRowWithMetrics wraps db.QueryRow with rebind
func QueryRowWithMetrics(db *sql.DB, queryType, query string, args ...interface{}) *sql.Row {
	return db.QueryRow(rebind(query), args...)
}

// QueryRowWithMetricsAndError wraps db.QueryRow
func QueryRowWithMetricsAndError(db *sql.DB, queryType, query string, args ...interface{}) (*sql.Row, func(error)) {
	row := db.QueryRow(rebind(query), args...)
	errorCallback := func(scanErr error) {}
	return row, errorCallback
}

// QueryRowWithMetricsAndErrorTx wraps tx.QueryRow
func QueryRowWithMetricsAndErrorTx(tx *sql.Tx, queryType, query string, args ...interface{}) (*sql.Row, func(error)) {
	row := tx.QueryRow(rebind(query), args...)
	errorCallback := func(scanErr error) {}
	return row, errorCallback
}
