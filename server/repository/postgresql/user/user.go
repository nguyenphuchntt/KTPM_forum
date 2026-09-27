package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"forum/server/database"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// SessionCookieName is the cookie that carries the session id.
const SessionCookieName = "session_id"

// StoreSession replaces the session row of an account. An account has at most
// one session, so the ON CONFLICT upsert keeps the UNIQUE(user_id) invariant.
func StoreSession(db *sql.DB, userID uuid.UUID, sessionID string, expiresAt time.Time) error {
	query := `INSERT INTO sessions (user_id, session_id, expires_at) VALUES (?,?,?)
		ON CONFLICT (user_id) DO UPDATE SET session_id = EXCLUDED.session_id, expires_at = EXCLUDED.expires_at`

	_, err := database.ExecWithMetrics(db, "insert_session", query, userID, sessionID, expiresAt)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}

// ValidSession resolves the session cookie of a request to an account. It is
// false when the cookie is missing, unknown or expired.
func ValidSession(r *http.Request, db *sql.DB) (uuid.UUID, string, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie == nil || cookie.Value == "" {
		return uuid.Nil, "", false
	}

	query := `
			SELECT
				s.user_id,
				s.expires_at,
				a.username
			FROM sessions s
			INNER JOIN accounts a ON s.user_id = a.id
			WHERE s.session_id = ? AND s.expires_at > NOW()
		`

	var (
		userID     uuid.UUID
		expiration time.Time
		username   string
	)

	row, recordError := database.QueryRowWithMetricsAndError(db, "select_session", query, cookie.Value)
	err = row.Scan(&userID, &expiration, &username)
	recordError(err)
	if err != nil {
		return uuid.Nil, "", false
	}

	return userID, username, true
}

// DeleteUserSession removes the account's session, signing the user out
// everywhere. Deleting nothing is not an error.
func DeleteUserSession(db *sql.DB, userID uuid.UUID) error {
	_, err := database.ExecWithMetrics(db, "delete_session", `DELETE FROM sessions WHERE user_id = ?;`, userID)
	return err
}

// Account is the account row needed by the auth usecase's Me endpoint.
type Account struct {
	ID       uuid.UUID
	Email    string
	Username string
	Role     string
}

// GetAccountByID loads the account behind a session user id.
// It returns sql.ErrNoRows when the id does not exist.
func GetAccountByID(db *sql.DB, userID uuid.UUID) (Account, error) {
	var account Account
	row, recordError := database.QueryRowWithMetricsAndError(
		db, "select_account_by_id",
		`SELECT id, email, username, role FROM accounts WHERE id = ?`, userID,
	)
	err := row.Scan(&account.ID, &account.Email, &account.Username, &account.Role)
	recordError(err)
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

// UsernameExists reports whether the username is already registered.
func UsernameExists(db *sql.DB, username string) bool {
	var exists bool
	row, recordError := database.QueryRowWithMetricsAndError(
		db, "username_exists",
		`SELECT EXISTS(SELECT 1 FROM accounts WHERE username = ?)`, username,
	)
	if err := row.Scan(&exists); err != nil {
		recordError(err)
		return false
	}
	return exists
}

// EmailExists reports whether the email is already registered.
func EmailExists(db *sql.DB, email string) bool {
	var exists bool
	row, recordError := database.QueryRowWithMetricsAndError(
		db, "email_exists",
		`SELECT EXISTS(SELECT 1 FROM accounts WHERE email = ?)`, email,
	)
	if err := row.Scan(&exists); err != nil {
		recordError(err)
		return false
	}
	return exists
}

// Credentials is what signin needs: the account id and its password hash.
type Credentials struct {
	UserID   uuid.UUID
	Password string
}

// ErrAccountNotFound is returned when a username has no account.
var ErrAccountNotFound = errors.New("account not found")

// GetCredentialsByUsername loads the id and password hash used to verify a signin.
func GetCredentialsByUsername(db *sql.DB, username string) (Credentials, error) {
	var credentials Credentials
	row, recordError := database.QueryRowWithMetricsAndError(
		db, "select_user_credentials",
		`SELECT id, password FROM accounts WHERE username = ?`, username,
	)
	err := row.Scan(&credentials.UserID, &credentials.Password)
	recordError(err)
	if errors.Is(err, sql.ErrNoRows) {
		return Credentials{}, ErrAccountNotFound
	}
	if err != nil {
		return Credentials{}, err
	}
	return credentials, nil
}

// StoreUser hashes the password, inserts the account and returns the new id.
// The id is generated here rather than left to the column default so the caller
// never needs a follow-up SELECT.
func StoreUser(db *sql.DB, email, username, password string) (uuid.UUID, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, err
	}

	accountID := uuid.New()
	query := `INSERT INTO accounts (id, email, username, password) VALUES (?,?,?,?)`
	if _, err := database.ExecWithMetrics(db, "insert_user", query, accountID, email, username, hashedPassword); err != nil {
		return uuid.Nil, fmt.Errorf("%v", err)
	}

	return accountID, nil
}
