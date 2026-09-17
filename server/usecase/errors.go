package usecase

import "net/http"

type AppError struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
}

func (e *AppError) Error() string { return e.Message }

func NewAppError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func NewValidationError(message string, details map[string]string) *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "validation_error",
		Message: message,
		Details: details,
	}
}

const (
	CodeValidationError = "validation_error"
	CodeUnauthorized    = "unauthorized"
	CodeForbidden       = "forbidden"
	CodeNotFound        = "not_found"
	CodeConflict        = "conflict"
	CodeInternal        = "internal_error"
)

// ToHTTP converts any error returned by a use case into the status, code,
// message and field details the controller should write.
func ToHTTP(err error) (int, string, string, map[string]string) {
	if err == nil {
		return http.StatusOK, "", "", nil
	}

	if appErr, ok := err.(*AppError); ok {
		return appErr.Status, appErr.Code, appErr.Message, appErr.Details
	}

	return http.StatusInternalServerError, CodeInternal, "internal server error", nil
}
