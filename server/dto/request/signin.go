package request

// SigninRequest is the payload for a signin attempt.
// It is bound from either a form-encoded body or a JSON body.
type SigninRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
