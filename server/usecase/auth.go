package usecase

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"forum/server/cache"
	"forum/server/config"
	"forum/server/dto/request"
	"forum/server/dto/response"
	userRepository "forum/server/repository/mysql/user"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const SessionTTL = 10 * time.Hour
const SessionCookieName = "session_id"

var (

	ErrInvalidCredentials = NewAppError(http.StatusUnauthorized, CodeUnauthorized, "invalid username or password")

	ErrNoSession         = NewAppError(http.StatusUnauthorized, CodeUnauthorized, "authentication required")
	ErrSessionGeneration = NewAppError(http.StatusInternalServerError, CodeInternal, "failed to generate session")
	ErrSessionStorage    = NewAppError(http.StatusInternalServerError, CodeInternal, "failed to store session")
	ErrSessionDelete     = NewAppError(http.StatusInternalServerError, CodeInternal, "failed to remove session")

	ErrEmailAlreadyTaken    = NewAppError(http.StatusConflict, CodeConflict, "email is already registered")
	ErrUsernameAlreadyTaken = NewAppError(http.StatusConflict, CodeConflict, "username is already taken")

	ErrEmailInvalid       = NewAppError(http.StatusBadRequest, CodeValidationError, "invalid email")
	ErrUsernameInvalid    = NewAppError(http.StatusBadRequest, CodeValidationError, "invalid username")
	ErrPasswordInvalid    = NewAppError(http.StatusBadRequest, CodeValidationError, "invalid password")
	ErrPasswordMismatch   = NewAppError(http.StatusBadRequest, CodeValidationError, "passwords do not match")
	ErrUserNotFound       = NewAppError(http.StatusNotFound, CodeNotFound, "user not found")
	ErrCredentialsFailure = NewAppError(http.StatusInternalServerError, CodeInternal, "failed to process credentials")
)

// AuthUsecase owns registration, signin, session validation and signout.
// It never touches http.ResponseWriter: it returns DTOs plus errors that carry
// their own HTTP status, and the controller turns those into a response.
type AuthUsecase struct {
	db *sql.DB
}

func NewAuthUsecase(db *sql.DB) *AuthUsecase {
	return &AuthUsecase{db: db}
}

// Signup registers a new account.
func (uc *AuthUsecase) Signup(req request.SignupRequest) (*response.SignupResponse, error) {
	email := strings.TrimSpace(req.Email)
	username := strings.TrimSpace(req.Username)

	userID, err := userRepository.StoreUser(uc.db, email, username, req.Password)
	if err != nil {
		if isDuplicateError(err) {
			// StoreUser does not tell us which unique key collided, so probe both
			// to give the caller a useful field-level message.
			if userRepository.UsernameExists(uc.db, username) {
				return nil, ErrUsernameAlreadyTaken
			}
			if userRepository.EmailExists(uc.db, email) {
				return nil, ErrEmailAlreadyTaken
			}
			return nil, ErrUsernameAlreadyTaken
		}
		return nil, ErrCredentialsFailure
	}

	return &response.SignupResponse{
		UserID:   userID.String(),
		Username: username,
		Email:    email,
	}, nil
}

// Signin verifies credentials and creates a session row. The returned DTO
// carries the session id and expiry that the controller writes into a cookie.
func (uc *AuthUsecase) Signin(req request.SigninRequest) (*response.SigninResponse, error) {
	username := strings.TrimSpace(req.Username)

	if len(username) < 4 {
		return nil, ErrUsernameInvalid
	}
	if len(req.Password) < 6 {
		return nil, ErrPasswordInvalid
	}

	credentials, err := userRepository.GetCredentialsByUsername(uc.db, username)
	if err != nil {
		if errors.Is(err, userRepository.ErrAccountNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, ErrCredentialsFailure
	}

	if err := bcrypt.CompareHashAndPassword([]byte(credentials.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	sessionID, err := config.GenerateSessionID()
	if err != nil {
		return nil, ErrSessionGeneration
	}

	expiresAt := time.Now().Add(SessionTTL)
	if err := userRepository.StoreSession(uc.db, credentials.UserID, sessionID, expiresAt); err != nil {
		return nil, ErrSessionStorage
	}

	if cache.GlobalSessionCache != nil {
		cache.GlobalSessionCache.Set(sessionID, credentials.UserID, username, expiresAt)
	}

	return &response.SigninResponse{
		UserID:    credentials.UserID.String(),
		Username:  username,
		SessionID: sessionID,
		ExpiresAt: expiresAt,
	}, nil
}

// Me returns the profile of the signed-in account.
func (uc *AuthUsecase) Me(userID uuid.UUID) (*response.MeResponse, error) {
	if userID == uuid.Nil {
		return nil, ErrNoSession
	}

	account, err := userRepository.GetAccountByID(uc.db, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, ErrCredentialsFailure
	}

	return &response.MeResponse{
		UserID:   account.ID.String(),
		Username: account.Username,
		Email:    account.Email,
	}, nil
}

// Logout drops the user's session row and evicts it from the session cache.
// Signing out is idempotent: an already expired session is not an error.
func (uc *AuthUsecase) Logout(userID uuid.UUID) error {
	if userID == uuid.Nil {
		return ErrNoSession
	}

	if err := userRepository.DeleteUserSession(uc.db, userID); err != nil {
		return ErrSessionDelete
	}

	if cache.GlobalSessionCache != nil {
		cache.GlobalSessionCache.DeleteByUserID(userID)
	}

	return nil
}

// SessionInfo is the resolved identity of the current request.
type SessionInfo struct {
	UserID   uuid.UUID
	Username string
	Valid    bool
}

// ValidateSession reads the session cookie from the request and returns the
// identity behind it. An invalid or missing cookie yields Valid == false rather
// than an error, because most callers only branch on authentication.
func (uc *AuthUsecase) ValidateSession(r *http.Request) SessionInfo {
	if r == nil {
		return SessionInfo{}
	}

	userID, username, valid := userRepository.ValidSession(r, uc.db)
	if !valid {
		return SessionInfo{}
	}

	return SessionInfo{UserID: userID, Username: username, Valid: true}
}

// RequireSession is the erroring variant: it returns ErrNoSession when the
// caller is anonymous, so usecases can fail fast without repeating the check.
func (uc *AuthUsecase) RequireSession(r *http.Request) (SessionInfo, error) {
	session := uc.ValidateSession(r)
	if !session.Valid {
		return SessionInfo{}, ErrNoSession
	}
	return session, nil
}

// SessionCookie builds the cookie for a freshly created session.
func SessionCookie(sessionID string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearSessionCookie is the cookie to send when signing out: same name and
// path, already expired, so the browser drops it immediately.
func ClearSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// isDuplicateError reports whether err is MySQL's unique-key violation (1062).
func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "error 1062") ||
		strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "is not unique")
}

func isAlphanumeric(s string) bool {
	for _, r := range s {
		isDigit := r >= '0' && r <= '9'
		isUpper := r >= 'A' && r <= 'Z'
		isLower := r >= 'a' && r <= 'z'
		if !isDigit && !isUpper && !isLower {
			return false
		}
	}
	return len(s) > 0
}
