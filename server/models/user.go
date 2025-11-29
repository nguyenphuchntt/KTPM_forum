package models

import (
	"database/sql"
	"fmt"

	"forum/server/database"
	"golang.org/x/crypto/bcrypt"
)

func GetUserInfo(db *sql.DB, username string) (int, string, error) {
	var user_id int
	var hashedPassword string
	row, recordError := database.QueryRowWithMetricsAndError(db, "select_user", "SELECT id,password FROM users WHERE username = ?", username)
	err := row.Scan(&user_id, &hashedPassword)
	recordError(err)
	if err != nil {
		return 0, "", err
	}
	return user_id, hashedPassword, nil
}

func StoreUser(db *sql.DB, email, username, password string) (int64, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return -1, err
	}

	query := `INSERT INTO users (email,username,password) VALUES (?,?,?)`
	result, err := database.ExecWithMetrics(db, "insert_user", query, email, username, hashedPassword)
	if err != nil {
		return -1, fmt.Errorf("%v", err)
	}

	userID, _ := result.LastInsertId()

	return userID, nil
}
