package models

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"forum/server/database"
)

func StoreSession(db *sql.DB, user_id int, session_id string, expires_at time.Time) error {
	query := `REPLACE INTO sessions (user_id,session_id,expires_at) VALUES (?,?,?)`

	_, err := database.ExecWithMetrics(db, "insert_session", query, user_id, session_id, expires_at)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	return nil
}

func ValidSession(r *http.Request, db *sql.DB) (int, string, bool) {
	cookie, err := r.Cookie("session_id")
	if err != nil || cookie == nil {
		return -1, "", false
	}
	var expiration time.Time
	var user_id int
	var username string
	query := `
		SELECT 
			s.user_id,
			s.expires_at, 
			u.username 
		FROM sessions s 
		INNER JOIN users u ON s.user_id = u.id 
		WHERE session_id = ?
	`
	row, recordError := database.QueryRowWithMetricsAndError(db, "select_session", query, cookie.Value)
	err = row.Scan(&user_id, &expiration, &username)
	recordError(err)
	if err != nil || expiration.Before(time.Now()) {
		return -1, "", false
	}
	return user_id, username, true
}

func DeleteUserSession(db *sql.DB, userID int) error {
	_, err := database.ExecWithMetrics(db, "delete_session", `DELETE FROM sessions WHERE user_id = ?;`, userID)
	return err
}
