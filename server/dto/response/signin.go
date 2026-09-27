package response

import "time"

// SigninResponse is returned after a successful signin.
// SessionID and ExpiresAt are also written to the session cookie by the controller.
type SigninResponse struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	SessionID string    `json:"session_id"`
	ExpiresAt time.Time `json:"expires_at"`
}
