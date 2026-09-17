package response

type MeResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type LogoutResponse struct {
	Message string `json:"message"`
}
