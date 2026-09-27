package request

// SignupRequest is the payload for a registration attempt.
// It is bound from either a form-encoded body or a JSON body.
type SignupRequest struct {
	Email                string `json:"email"`
	Username             string `json:"username"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

// LogoutRequest carries no payload; it is declared so the logout endpoint has a
// stable request type and can be extended later.
type LogoutRequest struct{}
