package controllers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"database/sql"

	"forum/server/config"
	"forum/server/utils"
)

// ServeStaticFiles returns a handler function for serving static files
func ServeStaticFiles(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// Get clean file path and prevent directory traversal
	filePath := filepath.Clean(config.BasePath + "web/assets" + strings.TrimPrefix(r.URL.Path, "/assets"))

	// block access to dirictories
	if info, err := os.Stat(filePath); err != nil || info.IsDir() {
		utils.RenderError(db, w, r, http.StatusNotFound, false, "")
		return
	}

	// Set proper MIME type for JavaScript modules
	if strings.HasSuffix(filePath, ".js") {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}

	// Serve the file
	http.ServeFile(w, r, filePath)
}
