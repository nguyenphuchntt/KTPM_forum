package controllers

import (
	"database/sql"
	"encoding/json"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"

	"fmt"
	"forum/server/cache"
	"forum/server/config"

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
	var valid bool
	var username string
	_, username, valid = models.ValidSession(r, db)

	if r.URL.Path != "/" || r.Method != http.MethodGet {
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

	cacheKey := "index_posts_page_" + strconv.Itoa(page)

	posts, found := getPostFromCache(cacheKey)
	if found {
		log.Println("Cache hit:", cacheKey)
		if err := utils.RenderTemplate(db, w, r, "home", http.StatusOK, posts, valid, username); err != nil {
			log.Println("Error rendering template:", err)
			utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		}
		return
	}

	log.Println("Cache miss:", cacheKey)
	
	// If cache miss 
	posts, statusCode, err := models.FetchPosts(db, page)
	if err != nil {
		log.Println("Error fetching posts:", err)
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}

	if posts == nil && page > 0 {
		utils.RenderError(db, w, r, 404, valid, username)
		return
	}

	if err == nil && posts != nil {
		cache.AppCache.Set(cacheKey, posts, config.CacheTTL)
	}

	if err := utils.RenderTemplate(db, w, r, "home", statusCode, posts, valid, username); err != nil {
		log.Println("Error rendering template:", err)
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
}

func IndexPostsByCategory(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var valid bool
	var username string
	_, username, valid = models.ValidSession(r, db)

	if r.Method != http.MethodGet {
		utils.RenderError(db, w, r, http.StatusMethodNotAllowed, valid, username)
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		utils.RenderError(db, w, r, http.StatusBadRequest, valid, username)
		return
	}

	if e := models.CheckCategories(db, []int{id}); e != nil {
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
		log.Println("Cache hit:", cacheKey)
		if err := utils.RenderTemplate(db, w, r, "home", http.StatusOK, posts, valid, username); err != nil {
			log.Println("Error rendering template:", err)
			utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		}
		return
	}
	log.Println("Cache miss:", cacheKey)

	// If cache miss
	posts, statusCode, err := models.FetchPostsByCategory(db, id, page)
	if err != nil {
		log.Println("Error fetching posts:", err)
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}

	if posts == nil && page > 0 {
		utils.RenderError(db, w, r, 404, valid, username)
		return
	}

	if err == nil && posts != nil {
        cache.AppCache.Set(cacheKey, posts, config.CacheTTL)
    }

	if err := utils.RenderTemplate(db, w, r, "home", statusCode, posts, valid, username); err != nil {
		log.Println("Error rendering template:", err)
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
}

func ShowPost(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var valid bool
	var username string
	_, username, valid = models.ValidSession(r, db)

	if r.Method != http.MethodGet {
		utils.RenderError(db, w, r, http.StatusMethodNotAllowed, valid, username)
		return
	}
	postID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		utils.RenderError(db, w, r, http.StatusBadRequest, valid, username)
		return
	}

	// Cache logic removed to fix 500 error (type mismatch between Post and PostDetail)
	// and to ensure comments are always fresh.
	// cacheKey := "post_" + strconv.Itoa(postID)
	// posts, found := getPostFromCache(cacheKey)
	// if found { ... }

	postDetail, statusCode, err := models.FetchPost(db, postID)
	if err != nil {
		log.Println("Error fetching posts from the database:", err)
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}

	// if err == nil && postDetail.Post.ID != 0 {
	// 	cache.AppCache.Set(cacheKey, []models.Post{postDetail.Post}, config.CacheTTL)
	// }

	err = utils.RenderTemplate(db, w, r, "post", statusCode, postDetail, valid, username)
	if err != nil {
		log.Println(err)
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
	}
}

func GetPostCreationForm(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var valid bool
	var username string

	if _, username, valid = models.ValidSession(r, db); !valid {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if r.Method != http.MethodGet {
		utils.RenderError(db, w, r, http.StatusMethodNotAllowed, valid, username)
		return
	}

	if err := utils.RenderTemplate(db, w, r, "post-form", http.StatusOK, nil, valid, username); err != nil {
		log.Println("Error rendering template:", err)
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
}

func CreatePost(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var user_id int
	var valid bool

	if user_id, _, valid = models.ValidSession(r, db); !valid {
		w.WriteHeader(401)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
		log.Println("Error parsing multipart form:", err)
		w.WriteHeader(400)
		return
	}

	title := r.FormValue("title")
	content := r.FormValue("content")
	catids := r.Form["categories"]
	
	// Handle categories (might be comma separated string in one element or multiple elements)
	if len(catids) > 0 && strings.Contains(catids[0], ",") {
		catids = strings.Split(catids[0], ",")
	}

	title = html.EscapeString(title)
	content = html.EscapeString(content)

	if len(catids) == 0 || strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		w.WriteHeader(400)
		return
	}

	var catidsInt []int
	for i := range catids {
		id, e := strconv.Atoi(catids[i])
		if e != nil {
			w.WriteHeader(400)
			return
		}
		catidsInt = append(catidsInt, id)
	}

	err := models.CheckCategories(db, catidsInt)
	if err != nil {
		w.WriteHeader(400)
		return
	}

	// Handle Image Upload
	var imagePath string
	
	// Check for pre-uploaded image URL (from Azure)
	imageUrl := r.FormValue("image_url")
	if imageUrl != "" {
		imagePath = imageUrl
	} else {
		// Fallback to local upload
		file, header, err := r.FormFile("image")
		if err == nil {
			defer file.Close()
			storage := utils.NewLocalStorage(config.BasePath + "web/assets/uploads")
			imagePath, err = storage.Save(file, header)
			if err != nil {
				log.Println("Error saving image:", err)
				w.WriteHeader(500)
				return
			}
		} else if err != http.ErrMissingFile {
			log.Println("Error retrieving image:", err)
			w.WriteHeader(400)
			return
		}
	}

	pid, err := models.StorePost(db, user_id, title, content, imagePath)
	if err != nil {
		log.Println("Error storing post:", err)
		w.WriteHeader(400)
		return
	}

	_, err = models.StoreAllPostCategories(db, pid, catidsInt)

	cache.AppCache.Delete("index_posts_page_0")
	for i := 0; i < len(catidsInt); i++ {
		cache.AppCache.Delete("category_posts_" + strconv.Itoa(catidsInt[i]) + "_page_0")
	}

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
	posts, statusCode, err := models.FetchCreatedPostsByUser(db, user_id, page)
	if err != nil {
		log.Println("Error fetching posts:", err)
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}
	if posts == nil && page > 0 {
		utils.RenderError(db, w, r, 404, valid, username)
		return
	}

	if err := utils.RenderTemplate(db, w, r, "home", statusCode, posts, valid, username); err != nil {
		log.Println("Error rendering template:", err)
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
	posts, statusCode, err := models.FetchLikedPostsByUser(db, user_id, page)
	if err != nil {
		log.Println("Error fetching posts:", err)
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}
	if posts == nil && page > 0 {
		utils.RenderError(db, w, r, 404, valid, username)
		return
	}

	if err := utils.RenderTemplate(db, w, r, "home", statusCode, posts, valid, username); err != nil {
		log.Println("Error rendering template:", err)
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
