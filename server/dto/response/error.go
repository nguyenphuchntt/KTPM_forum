package response

// Error codes shared by the JSON API handlers.
const (
	CodeValidationError = "validation_error"
	CodeUnauthorized    = "unauthorized"
	CodeNotFound        = "not_found"
	CodeConflict        = "conflict"
	CodeInternal        = "internal_error"
)

// ErrorBody is the stable error object returned by JSON endpoints.
type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// ErrorResponse is the envelope for every JSON error response.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// NewError builds an error envelope, omitting details when there are none.
func NewError(code, message string, details map[string]string) ErrorResponse {
	return ErrorResponse{Error: ErrorBody{
		Code:    code,
		Message: message,
		Details: details,
	}}
}
