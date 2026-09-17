package controllers

import (
	"net/http"

	"forum/server/usecase"
)

// AuthController is the HTTP transport for the auth usecase. It holds no
// business logic: it binds, validates, calls the usecase and writes the DTO.
type AuthController struct {
	Auth *usecase.AuthUsecase
}

func NewAuthController(auth *usecase.AuthUsecase) AuthController {
	return AuthController{Auth: auth}
}

// RegisterAuthRoutes mounts the JSON auth endpoints on the given mux.
func RegisterAuthRoutes(mux *http.ServeMux, c AuthController) {
	mux.HandleFunc("/api/v1/auth/signin", c.SigninJSON)
	mux.HandleFunc("/api/v1/auth/signup", c.SignupJSON)
	mux.HandleFunc("/api/v1/auth/logout", c.LogoutJSON)
	mux.HandleFunc("/api/v1/auth/me", c.MeJSON)
}
