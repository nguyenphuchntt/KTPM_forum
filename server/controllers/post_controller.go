package controllers

import (
	"database/sql"
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fmt"
	"forum/server/cache"
	"forum/server/config"
	"forum/server/logger"
	"forum/server/models"
	"forum/server/utils"
)

func getPostFromCache(cacheKey string) ([]models.Post, bool) {
	cachedData, found := cache.AppCache.Get(cacheKey)
	if !found {
		return nil, false
	}

	posts, ok := cachedData.([]models.Post)
	if !ok {
		cache.AppCache.Delete(cacheKey)
		return nil, false
	}

	return posts, true
}

func IndexPosts(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	start := time.Now()
	
	var valid bool
	var username string
	userID, username, valid := models.ValidSession(r, db)

	log := logger.WithRequest(r, userID)
	log.Info().Msg("Fetching posts")

	if r.URL.Path != "/" || r.Method != http.MethodGet {
		log.Warn().Int("status", http.StatusNotFound).Msg("Invalid path or method")
		utils.RenderError(db, w, r, http.StatusNotFound, valid, username)
		return
	}
	id := r.FormValue("PageID")
	page, er := strconv.Atoi(id)
	if er != nil && id != "" {
		log.Warn().Str("page_id", id).Msg("Invalid page ID")
		utils.RenderError(db, w, r, http.StatusBadRequest, valid, username)
		return
	}
	page = (page - 1) * 10
	if page < 0 {
		page = 0
	}

	cacheKey := "index_posts_page_" + strconv.Itoa(page)

	posts, found := getPostFromCache(cacheKey)
	if found {
		log.Info().
			Str("cache_key", cacheKey).
			Bool("cache_hit", true).
			Int("post_count", len(posts)).
			Dur("duration_ms", time.Since(start)).
			Msg("Posts fetched from cache")
		if err := utils.RenderTemplate(db, w, r, "home", http.StatusOK, posts, valid, username); err != nil {
			log.Error().Err(err).Msg("Error rendering template")
			utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		}
		return
	}

	log.Debug().Str("cache_key", cacheKey).Msg("Cache miss")
	
	// If cache miss 
	posts, statusCode, err := models.FetchPosts(db, page)
	if err != nil {
		log.Error().
			Err(err).
			Int("status", statusCode).
			Int("page", page).
			Dur("duration_ms", time.Since(start)).
			Msg("Failed to fetch posts")
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}

	if posts == nil && page > 0 {
		log.Warn().Int("page", page).Msg("No posts found for page")
		utils.RenderError(db, w, r, 404, valid, username)
		return
	}

	if err == nil && posts != nil {
		cache.AppCache.Set(cacheKey, posts, config.CacheTTL)
	}

	if err := utils.RenderTemplate(db, w, r, "home", statusCode, posts, valid, username); err != nil {
		log.Error().Err(err).Msg("Error rendering template")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}

	log.Info().
		Int("post_count", len(posts)).
		Int("page", page).
		Bool("cache_hit", false).
		Dur("duration_ms", time.Since(start)).
		Msg("Posts fetched successfully from database")
}

func IndexPostsByCategory(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	start := time.Now()
	
	var valid bool
	var username string
	userID, username, valid := models.ValidSession(r, db)
	
	log := logger.WithRequest(r, userID)

	if r.Method != http.MethodGet {
		log.Warn().Msg("Invalid method for category posts")
		utils.RenderError(db, w, r, http.StatusMethodNotAllowed, valid, username)
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		log.Warn().Str("category_id", r.PathValue("id")).Msg("Invalid category ID")
		utils.RenderError(db, w, r, http.StatusBadRequest, valid, username)
		return
	}

	if e := models.CheckCategories(db, []int{id}); e != nil {
		log.Warn().Int("category_id", id).Msg("Category not found")
		utils.RenderError(db, w, r, 404, valid, username)
		return
	}

	pid := r.FormValue("PageID")
	page, _ := strconv.Atoi(pid)
	page = (page - 1) * 10
	if page < 0 {
		page = 0
	}

	cacheKey := fmt.Sprintf("category_posts_%d_page_%d", id, int(page))

	posts, found := getPostFromCache(cacheKey)
	if found {
		log.Info().
			Str("cache_key", cacheKey).
			Bool("cache_hit", true).
			Int("post_count", len(posts)).
			Dur("duration_ms", time.Since(start)).
			Msg("Category posts fetched from cache")
		if err := utils.RenderTemplate(db, w, r, "home", http.StatusOK, posts, valid, username); err != nil {
			log.Error().Err(err).Msg("Error rendering template")
			utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		}
		return
	}
	log.Debug().Str("cache_key", cacheKey).Msg("Cache miss")

	// If cache miss
	posts, statusCode, err := models.FetchPostsByCategory(db, id, page)
	if err != nil {
		log.Error().
			Err(err).
			Int("category_id", id).
			Int("status", statusCode).
			Dur("duration_ms", time.Since(start)).
			Msg("Failed to fetch category posts")
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}

	if posts == nil && page > 0 {
		log.Warn().Int("page", page).Int("category_id", id).Msg("No posts found for category page")
		utils.RenderError(db, w, r, 404, valid, username)
		return
	}

	if err == nil && posts != nil {
		cache.AppCache.Set(cacheKey, posts, config.CacheTTL)
	}

	if err := utils.RenderTemplate(db, w, r, "home", statusCode, posts, valid, username); err != nil {
		log.Error().Err(err).Msg("Error rendering template")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
	
	log.Info().
		Int("post_count", len(posts)).
		Int("category_id", id).
		Int("page", page).
		Bool("cache_hit", false).
		Dur("duration_ms", time.Since(start)).
		Msg("Category posts fetched successfully")
}

func ShowPost(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	start := time.Now()
	
	var valid bool
	var username string
	userID, username, valid := models.ValidSession(r, db)
	
	log := logger.WithRequest(r, userID)

	if r.Method != http.MethodGet {
		log.Warn().Msg("Invalid method for show post")
		utils.RenderError(db, w, r, http.StatusMethodNotAllowed, valid, username)
		return
	}
	postID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		log.Warn().Str("post_id", r.PathValue("id")).Msg("Invalid post ID")
		utils.RenderError(db, w, r, http.StatusBadRequest, valid, username)
		return
	}

	cacheKey := "post_" + strconv.Itoa(postID)

	// Try to get PostDetail from cache (changed from []Post to PostDetail)
	cachedData, found := cache.AppCache.Get(cacheKey)
	if found {
		postDetail, ok := cachedData.(models.PostDetail)
		if ok {
			log.Info().
				Int("post_id", postID).
				Bool("cache_hit", true).
				Dur("duration_ms", time.Since(start)).
				Msg("Post fetched from cache")
			if err := utils.RenderTemplate(db, w, r, "post", http.StatusOK, postDetail, valid, username); err != nil {
				log.Error().Err(err).Msg("Error rendering template")
				utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
				return
			}
			return
		}
		// If type assertion failed, delete invalid cache
		cache.AppCache.Delete(cacheKey)
	}

	// If cache miss
	log.Debug().Str("cache_key", cacheKey).Msg("Cache miss")

	postDetail, statusCode, err := models.FetchPost(db, postID)
	if err != nil {
		log.Error().
			Err(err).
			Int("post_id", postID).
			Int("status", statusCode).
			Dur("duration_ms", time.Since(start)).
			Msg("Failed to fetch post")
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}

	// Cache the entire PostDetail, not just the Post
	if err == nil && postDetail.Post.ID != 0 {
		cache.AppCache.Set(cacheKey, postDetail, config.CacheTTL)
	}

	err = utils.RenderTemplate(db, w, r, "post", statusCode, postDetail, valid, username)
	if err != nil {
		log.Error().Err(err).Msg("Error rendering post template")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
	
	log.Info().
		Int("post_id", postID).
		Bool("cache_hit", false).
		Dur("duration_ms", time.Since(start)).
		Msg("Post fetched successfully")
}

func GetPostCreationForm(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var valid bool
	var username string

	userID, username, valid := models.ValidSession(r, db)
	if !valid {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if r.Method != http.MethodGet {
		utils.RenderError(db, w, r, http.StatusMethodNotAllowed, valid, username)
		return
	}

	if err := utils.RenderTemplate(db, w, r, "post-form", http.StatusOK, nil, valid, username); err != nil {
		log := logger.WithRequest(r, userID)
		log.Error().Err(err).Msg("Error rendering post creation form")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
}

func CreatePost(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	start := time.Now()
	
	var user_id int
	var valid bool

	if user_id, _, valid = models.ValidSession(r, db); !valid {
		w.WriteHeader(401)
		return
	}
	
	log := logger.WithRequest(r, user_id)
	log.Info().Msg("Creating new post")

	if r.Method != http.MethodPost {
		log.Warn().Msg("Invalid method for create post")
		w.WriteHeader(405)
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Error().Err(err).Msg("Failed to parse form")
		w.WriteHeader(400)
		return
	}

	title := r.FormValue("title")
	content := r.FormValue("content")
	catids := r.Form["categories"]

	catids = strings.Split(catids[0], ",")

	title = html.EscapeString(title)
	content = html.EscapeString(content)

	if catids == nil || strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		log.Warn().
			Bool("empty_title", strings.TrimSpace(title) == "").
			Bool("empty_content", strings.TrimSpace(content) == "").
			Bool("no_categories", catids == nil).
			Msg("Invalid post data")
		w.WriteHeader(400)
		return
	}

	var catidsInt []int
	for i := range catids {
		id, e := strconv.Atoi(catids[i])
		if e != nil {
			log.Warn().Str("category_id", catids[i]).Msg("Invalid category ID format")
			w.WriteHeader(400)
			return
		}
		catidsInt = append(catidsInt, id)
	}

	err := models.CheckCategories(db, catidsInt)
	if err != nil {
		log.Warn().Ints("category_ids", catidsInt).Msg("Invalid categories")
		w.WriteHeader(400)
		return
	}

	pid, err := models.StorePost(db, user_id, title, content)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store post")
		w.WriteHeader(400)
		return
	}

	_, err = models.StoreAllPostCategories(db, pid, catidsInt)
	if err != nil {
		log.Error().Err(err).Int64("post_id", pid).Msg("Failed to store post categories")
		w.WriteHeader(400)
		return
	}

	// Invalidate cache
	cache.AppCache.Delete("index_posts_page_0")
	for i := 0; i < len(catidsInt); i++ {
		cache.AppCache.Delete("category_posts_" + strconv.Itoa(catidsInt[i]) + "_page_0")
	}

	log.Info().
		Int64("post_id", pid).
		Str("title", title).
		Ints("categories", catidsInt).
		Dur("duration_ms", time.Since(start)).
		Msg("Post created successfully")

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(200)
}

func MyCreatedPosts(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var valid bool
	var username string
	var user_id int
	if user_id, username, valid = models.ValidSession(r, db); !valid {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if r.Method != http.MethodGet {
		utils.RenderError(db, w, r, http.StatusNotFound, valid, username)
		return
	}
	id := r.FormValue("PageID")
	page, er := strconv.Atoi(id)
	if er != nil && id != "" {
		utils.RenderError(db, w, r, http.StatusBadRequest, valid, username)
		return
	}
	page = (page - 1) * 10
	if page < 0 {
		page = 0
	}
	log := logger.WithRequest(r, user_id)
	
	posts, statusCode, err := models.FetchCreatedPostsByUser(db, user_id, page)
	if err != nil {
		log.Error().Err(err).Int("page", page).Msg("Error fetching user created posts")
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}
	if posts == nil && page > 0 {
		utils.RenderError(db, w, r, 404, valid, username)
		return
	}

	if err := utils.RenderTemplate(db, w, r, "home", statusCode, posts, valid, username); err != nil {
		log.Error().Err(err).Msg("Error rendering template")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
}

func MyLikedPosts(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var valid bool
	var username string
	var user_id int
	if user_id, username, valid = models.ValidSession(r, db); !valid {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if r.Method != http.MethodGet {
		utils.RenderError(db, w, r, http.StatusNotFound, valid, username)
		return
	}
	id := r.FormValue("PageID")
	page, er := strconv.Atoi(id)
	if er != nil && id != "" {
		utils.RenderError(db, w, r, http.StatusBadRequest, valid, username)
		return
	}
	page = (page - 1) * 10
	if page < 0 {
		page = 0
	}
	log := logger.WithRequest(r, user_id)
	
	posts, statusCode, err := models.FetchLikedPostsByUser(db, user_id, page)
	if err != nil {
		log.Error().Err(err).Int("page", page).Msg("Error fetching user liked posts")
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}
	if posts == nil && page > 0 {
		utils.RenderError(db, w, r, 404, valid, username)
		return
	}

	if err := utils.RenderTemplate(db, w, r, "home", statusCode, posts, valid, username); err != nil {
		log.Error().Err(err).Msg("Error rendering template")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
}

func ReactToPost(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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
	id := r.FormValue("post_id")
	post_id, err := strconv.Atoi(id)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	likeCount, dislikeCount, err := models.ReactToPost(db, user_id, post_id, userReaction)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	// Return the new count as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"likesCount": likeCount, "dislikesCount": dislikeCount})
}
