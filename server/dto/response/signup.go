package response

// SignupResponse is returned after a successful registration.
type SignupResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}
