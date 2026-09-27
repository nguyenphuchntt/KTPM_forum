package validators

import (
	"encoding/json"
	"net/http"
	"strings"

	"forum/server/dto/request"
	"forum/server/utils"
)

// BindSigninRequest decodes a signin payload from a JSON body or a form body.
// Field-level validation is performed by ValidateSigninRequest.
func BindSigninRequest(r *http.Request) (request.SigninRequest, error) {
	var req request.SigninRequest

	if isJSON(r) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return req, err
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return req, err
		}
		req.Username = r.FormValue("username")
		req.Password = r.FormValue("password")
	}

	req.Username = strings.TrimSpace(req.Username)
	// Passwords are trimmed on both signup and signin so the stored hash and the
	// submitted value are normalized the same way. They are never HTML-escaped.
	req.Password = strings.TrimSpace(req.Password)

	return req, nil
}

// ValidateSigninRequest returns per-field errors keyed by JSON field name.
// A nil result means the payload is valid.
func ValidateSigninRequest(req request.SigninRequest) map[string]string {
	details := map[string]string{}

	if req.Username == "" {
		details["username"] = "username is required"
	} else if len(req.Username) < 4 {
		details["username"] = "username must be at least 4 characters long"
	}

	if req.Password == "" {
		details["password"] = "password is required"
	} else if len(req.Password) < 6 {
		details["password"] = "password must be at least 6 characters long"
	}

	if len(details) == 0 {
		return nil
	}
	return details
}

// BindSignupRequest decodes a signup payload from a JSON body or a form body.
func BindSignupRequest(r *http.Request) (request.SignupRequest, error) {
	var req request.SignupRequest

	if isJSON(r) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return req, err
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return req, err
		}
		req.Email = r.FormValue("email")
		req.Username = r.FormValue("username")
		req.Password = r.FormValue("password")
		req.PasswordConfirmation = r.FormValue("password-confirmation")
		if req.PasswordConfirmation == "" {
			req.PasswordConfirmation = r.FormValue("password_confirmation")
		}
	}

	req.Email = strings.TrimSpace(req.Email)
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.PasswordConfirmation = strings.TrimSpace(req.PasswordConfirmation)

	return req, nil
}

// ValidateSignupRequest returns per-field errors keyed by JSON field name.
// A nil result means the payload is valid.
func ValidateSignupRequest(req request.SignupRequest) map[string]string {
	details := map[string]string{}

	switch {
	case req.Email == "":
		details["email"] = "email is required"
	case !strings.Contains(req.Email, "@") || !strings.Contains(req.Email, ".") || len(req.Email) < 5:
		details["email"] = "invalid email format"
	}

	switch {
	case req.Username == "":
		details["username"] = "username is required"
	case len(req.Username) < 4:
		details["username"] = "username must be at least 4 characters long"
	case strings.Contains(req.Username, " "):
		details["username"] = "username cannot contain spaces"
	case !utils.IsAlphanumeric(req.Username):
		details["username"] = "username must contain only letters and numbers"
	}

	switch {
	case req.Password == "":
		details["password"] = "password is required"
	case req.Password != req.PasswordConfirmation:
		details["password_confirmation"] = "passwords do not match"
	case len(req.Password) < 6:
		details["password"] = "password must be at least 6 characters long"
	case !utils.ContainsUppercase(req.Password):
		details["password"] = "password must contain at least one uppercase letter"
	case !utils.ContainsDigit(req.Password):
		details["password"] = "password must contain at least one digit"
	}

	if len(details) == 0 {
		return nil
	}
	return details
}

func isJSON(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json")
}
