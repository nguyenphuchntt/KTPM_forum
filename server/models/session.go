package models

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"
	// "forum/server/utils/retry"
	"context"

	"forum/server/cache"
	// "forum/server/database"
)

func StoreSession(db *sql.DB, user_id int, session_id string, expires_at time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `REPLACE INTO sessions (user_id,session_id,expires_at) VALUES (?,?,?)`

	_, err := db.ExecContext(ctx, query, user_id, session_id, expires_at)
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

	// Try to get from cache first
	if cache.GlobalSessionCache != nil {
		if entry, found := cache.GlobalSessionCache.Get(cookie.Value); found {
			return entry.UserID, entry.Username, true
		}
	}

	// Not in cache, query database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
			SELECT 
				s.user_id,
				s.expires_at, 
				u.username 
			FROM sessions s 
			INNER JOIN users u ON s.user_id = u.id 
			WHERE session_id = ?
		`
	var expiration time.Time
	var user_id int
	var username string

	row := db.QueryRowContext(ctx, query, cookie.Value)
	err = row.Scan(&user_id, &expiration, &username)

	if err != nil || expiration.Before(time.Now()) {
		return -1, "", false
	}

	// Cache the valid session
	if cache.GlobalSessionCache != nil {
		cache.GlobalSessionCache.Set(cookie.Value, user_id, username, expiration)
	}

	return user_id, username, true
}

func DeleteUserSession(db *sql.DB, userID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Invalidate cache
	if cache.GlobalSessionCache != nil {
		cache.GlobalSessionCache.DeleteByUserID(userID)
	}

	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?;`, userID)
	return err
}
