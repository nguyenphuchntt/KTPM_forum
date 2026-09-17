package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"forum/server/dto/response"
	"forum/server/logger"
	"forum/server/usecase"
	"forum/server/validators"
)

// writeJSON writes a payload with an explicit status code.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// writeAppError maps a usecase error onto the shared JSON error envelope.
func writeAppError(w http.ResponseWriter, err error) {
	status, code, message, details := usecase.ToHTTP(err)
	writeJSON(w, status, response.NewError(code, message, details))
}

// bindAuthError reports whether the request body itself was malformed.
var errMalformedBody = errors.New("malformed request body")

// SigninJSON handles POST /api/v1/auth/signin.
func (c AuthController) SigninJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	req, err := validators.BindSigninRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "malformed request body", nil))
		return
	}

	if details := validators.ValidateSigninRequest(req); details != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid signin payload", details))
		return
	}

	result, err := c.Auth.Signin(req)
	if err != nil {
		writeAppError(w, err)
		return
	}

	http.SetCookie(w, usecase.SessionCookie(result.SessionID, result.ExpiresAt))
	log := logger.WithRequest(r, result.UserID)
	log.Info().Msg("User signed in")
	writeJSON(w, http.StatusOK, result)
}

// SignupJSON handles POST /api/v1/auth/signup.
func (c AuthController) SignupJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	req, err := validators.BindSignupRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "malformed request body", nil))
		return
	}

	if details := validators.ValidateSignupRequest(req); details != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid signup payload", details))
		return
	}

	result, err := c.Auth.Signup(req)
	if err != nil {
		writeAppError(w, err)
		return
	}

	log := logger.WithRequest(r, result.UserID)
	log.Info().Msg("User registered")
	writeJSON(w, http.StatusCreated, result)
}

// MeJSON handles GET /api/v1/auth/me. The account id comes from the session,
// never from the request body, so a caller cannot ask for someone else.
func (c AuthController) MeJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	session, err := c.Auth.RequireSession(r)
	if err != nil {
		writeAppError(w, err)
		return
	}

	result, err := c.Auth.Me(session.UserID)
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// LogoutJSON handles POST /api/v1/auth/logout and clears the session cookie.
func (c AuthController) LogoutJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	session, err := c.Auth.RequireSession(r)
	if err != nil {
		writeAppError(w, err)
		return
	}

	if err := c.Auth.Logout(session.UserID); err != nil {
		writeAppError(w, err)
		return
	}

	http.SetCookie(w, usecase.ClearSessionCookie())
	log := logger.WithRequest(r, session.UserID)
	log.Info().Msg("User signed out")
	writeJSON(w, http.StatusOK, response.LogoutResponse{Message: "signed out"})
}
