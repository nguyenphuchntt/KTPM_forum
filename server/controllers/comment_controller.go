package controllers

import (
	"database/sql"
	"encoding/json"
	"forum/server/cache"
	"html"
	"net/http"
	"strconv"
	"strings"

	"forum/server/models"
)

func CreateComment(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Validate session
	userID, username, valid := models.ValidSession(r, db)
	if !valid {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Validate method
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	content := html.EscapeString(strings.TrimSpace(r.FormValue("comment")))
	postIDStr := r.FormValue("postid")
	postID, err := strconv.Atoi(postIDStr)
	if err != nil || content == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// Store the comment using the models package
	commentID, err := models.StoreComment(db, userID, postID, content)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Invalidate cache (comment count changed)
	cache.AppCache.Delete("post_" + strconv.Itoa(postID))
	cache.AppCache.Delete("index_posts_page_0")

	// Fetch additional details using the models package
	commentsCount, err := models.CountCommentsByPostID(db, postID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	commentTime, err := models.FetchCommentTimeByID(db, commentID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	cache.AppCache.Delete("post_" + strconv.Itoa(postID))

	cache.AppCache.Delete("index_posts_page_0")

	// Return the new comment details as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ID":            commentID,
		"username":      username,
		"created_at":    commentTime,
		"content":       content,
		"likes":         0,
		"dislikes":      0,
		"commentscount": commentsCount,
	})
}

func ReactToComment(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user_id int
	var valid bool

	if user_id, _, valid = models.ValidSession(r, db); !valid {
		w.WriteHeader(401)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(400)
		return
	}

	userReaction := r.FormValue("reaction")
	id := r.FormValue("comment_id")
	comment_id, err := strconv.Atoi(id)
	if err != nil {
		w.WriteHeader(400)
		return
	}

	var post_id int
	err = db.QueryRow("SELECT post_id FROM comments WHERE id = ?", comment_id).Scan(&post_id)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	likeCount, dislikeCount, err := models.ReactToComment(db, user_id, comment_id, userReaction)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	cache.AppCache.Delete("post_" + strconv.Itoa(post_id))

	// Return the new count as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"commentlikesCount": likeCount, "commentdislikesCount": dislikeCount})
}
