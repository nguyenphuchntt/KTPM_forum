package controllers

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"forum/server/config"
	"forum/server/logger"
	userRepo "forum/server/repository/mysql/user"
	"forum/server/utils"

	"golang.org/x/crypto/bcrypt"
)

func GetLoginPage(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if _, _, valid := userRepo.ValidSession(r, db); valid {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method != http.MethodGet {
		utils.RenderError(db, w, r, http.StatusMethodNotAllowed, false, "")
		return
	}

	err := utils.RenderTemplate(db, w, r, "login", http.StatusOK, nil, false, "")
	if err != nil {
		log := logger.WithRequest(r, 0)
		log.Error().Err(err).Msg("Error rendering login page")
		http.Redirect(w, r, "/500", http.StatusSeeOther)
	}
}

func Signin(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	start := time.Now()
	log := logger.WithRequest(r, 0)

	if _, _, valid := userRepo.ValidSession(r, db); valid {
		log.Warn().Msg("User already logged in")
		w.WriteHeader(302)
		return
	}

	if r.Method != http.MethodPost {
		log.Warn().Msg("Invalid method for signin")
		w.WriteHeader(405)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Error().Err(err).Msg("Failed to parse login form")
		w.WriteHeader(400)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if len(username) < 4 || len(password) < 6 {
		log.Warn().
			Str("username", username).
			Bool("username_too_short", len(username) < 4).
			Bool("password_too_short", len(password) < 6).
			Msg("Invalid credentials format")
		w.WriteHeader(400)
		return
	}

	// get user credentials from database
	creds, err := userRepo.GetCredentialsByUsername(db, username)
	if err != nil {
		if errors.Is(err, userRepo.ErrAccountNotFound) || errors.Is(err, sql.ErrNoRows) {
			log.Warn().Str("username", username).Msg("User not found")
			w.WriteHeader(404)
			return
		}
		log.Error().Err(err).Str("username", username).Msg("Database error during login")
		w.WriteHeader(500)
		return
	}

	// Verify the password
	if err := bcrypt.CompareHashAndPassword([]byte(creds.Password), []byte(password)); err != nil {
		log.Warn().
			Str("username", username).
			Str("user_id", creds.UserID.String()).
			Msg("Invalid password attempt")
		w.WriteHeader(401)
		return
	}

	sessionID, err := config.GenerateSessionID()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate session ID")
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	err = userRepo.StoreSession(db, creds.UserID, sessionID, time.Now().Add(10*time.Hour))
	if err != nil {
		log.Error().Err(err).Str("user_id", creds.UserID.String()).Msg("Failed to store session")
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Set session ID as a cookie
	http.SetCookie(w, &http.Cookie{
		Name:    "session_id",
		Value:   sessionID,
		Expires: time.Now().Add(10 * time.Hour),
		Path:    "/",
	})

	log.Info().
		Str("username", username).
		Str("user_id", creds.UserID.String()).
		Dur("duration_ms", time.Since(start)).
		Msg("User logged in successfully")

	http.Redirect(w, r, "/", http.StatusFound)
}

func Logout(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if userID, username, valid := userRepo.ValidSession(r, db); valid {
		log := logger.WithRequest(r, 0)

		err := userRepo.DeleteUserSession(db, userID)
		if err != nil {
			log.Error().Err(err).Msg("Error deleting session during logout")
			http.Error(w, "Error while logging out!", http.StatusInternalServerError)
			return
		}

		log.Info().Str("username", username).Msg("User logged out successfully")

		w.Header().Set("Content-Type", "text/html")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}
