package controllers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"forum/server/cache"
	"forum/server/config"
	"forum/server/logger"
	"forum/server/model"
	categoryRepo "forum/server/repository/mysql/category"
	postRepo "forum/server/repository/mysql/post"
	userRepo "forum/server/repository/mysql/user"
	"forum/server/utils"
)

// cache post by its ID
func cachePost(post model.Post) {
	key := fmt.Sprintf("post_%d", post.ID)
	cache.AppCache.Set(key, post, config.CacheTTL)
}

func getCachedPostByID(postID int) (model.Post, bool) {
	key := fmt.Sprintf("post_%d", postID)
	data, found := cache.AppCache.Get(key)
	if !found {
		return model.Post{}, false
	}
	post, ok := data.(model.Post)
	if !ok {
		cache.AppCache.Delete(key)
		return model.Post{}, false
	}
	return post, true
}

func getCachedPosts(postIDs []int) map[model.PostID]model.Post {
	result := make(map[model.PostID]model.Post)
	for _, id := range postIDs {
		if post, found := getCachedPostByID(id); found {
			result[model.PostID(id)] = post
		}
	}
	return result
}

func getPostDetailFromCache(cacheKey string) (postRepo.PostDetail, bool) {
	cachedData, found := cache.AppCache.Get(cacheKey)
	if !found {
		return postRepo.PostDetail{}, false
	}

	postDetail, ok := cachedData.(postRepo.PostDetail)
	if !ok {
		cache.AppCache.Delete(cacheKey)
		return postRepo.PostDetail{}, false
	}

	return postDetail, true
}

func IndexPosts(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	userID, username, valid := userRepo.ValidSession(r, db)

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
	offset := (page - 1) * 10
	if offset < 0 {
		offset = 0
	}

	postIDs, _, err := postRepo.FetchPostIDsByTimestamp(db, offset, 10)
	if err != nil {
		log.Error().Err(err).Int("offset", offset).Msg("Failed to fetch post IDs")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}

	if len(postIDs) == 0 {
		if offset > 0 {
			log.Warn().Int("offset", offset).Msg("No posts found for offset")
			utils.RenderError(db, w, r, 404, valid, username)
			return
		}
		if err := utils.RenderTemplate(db, w, r, "home", http.StatusOK, []model.Post{}, valid, username); err != nil {
			log.Error().Err(err).Msg("Error rendering template")
			utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		}
		return
	}

	cachedPosts := getCachedPosts(postIDs)
	missingPostIDs := []int{}
	for _, id := range postIDs {
		if _, found := cachedPosts[model.PostID(id)]; !found {
			missingPostIDs = append(missingPostIDs, id)
		}
	}
	if len(missingPostIDs) > 0 {
		dbPosts, err := postRepo.FetchPostsByIDs(db, missingPostIDs)
		if err != nil {
			log.Error().Err(err).Ints("missing_ids", missingPostIDs).Msg("Failed to fetch missing posts")
		} else {
			for id, post := range dbPosts {
				cachedPosts[id] = post
				cachePost(post)
			}
		}
	}

	posts := make([]model.Post, 0, len(postIDs))
	for _, id := range postIDs {
		if post, found := cachedPosts[model.PostID(id)]; found {
			posts = append(posts, post)
		}
	}

	if err := utils.RenderTemplate(db, w, r, "home", http.StatusOK, posts, valid, username); err != nil {
		log.Error().Err(err).Msg("Error rendering template")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
}

func IndexPostsByCategory(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	userID, username, valid := userRepo.ValidSession(r, db)

	log := logger.WithRequest(r, userID)
	log.Info().Msg("Fetching posts by category")

	categoryIDStr := strings.TrimPrefix(r.URL.Path, "/category/")
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		log.Warn().Str("category_id", categoryIDStr).Msg("Invalid category ID format")
		utils.RenderError(db, w, r, http.StatusBadRequest, valid, username)
		return
	}

	if e := categoryRepo.CheckCategories(db, []int{categoryID}); e != nil {
		log.Warn().Int("category_id", categoryID).Msg("Category not found")
		utils.RenderError(db, w, r, http.StatusNotFound, valid, username)
		return
	}

	pageIDStr := r.FormValue("PageID")
	page, err := strconv.Atoi(pageIDStr)
	if err != nil && pageIDStr != "" {
		log.Warn().Str("page_id", pageIDStr).Msg("Invalid page ID format")
		utils.RenderError(db, w, r, http.StatusBadRequest, valid, username)
		return
	}

	offset := (page - 1) * 10
	if offset < 0 {
		offset = 0
	}

	postIDs, _, err := postRepo.FetchPostIDsForCategoryPage(db, categoryID, offset, 10)
	if err != nil {
		log.Error().Err(err).Int("category_id", categoryID).Int("offset", offset).Msg("Failed to fetch category post IDs")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}

	if len(postIDs) == 0 {
		if offset > 0 {
			log.Warn().Int("offset", offset).Msg("No posts found for category offset")
			utils.RenderError(db, w, r, 404, valid, username)
			return
		}
		if err := utils.RenderTemplate(db, w, r, "home", http.StatusOK, []model.Post{}, valid, username); err != nil {
			log.Error().Err(err).Msg("Error rendering template")
			utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		}
		return
	}

	cachedPosts := getCachedPosts(postIDs)
	missingIDs := []int{}
	for _, id := range postIDs {
		if _, found := cachedPosts[model.PostID(id)]; !found {
			missingIDs = append(missingIDs, id)
		}
	}

	if len(missingIDs) > 0 {
		dbPosts, err := postRepo.FetchPostsByIDs(db, missingIDs)
		if err != nil {
			log.Error().Err(err).Ints("missing_ids", missingIDs).Msg("Failed to fetch missing posts")
		} else {
			for id, post := range dbPosts {
				cachedPosts[id] = post
				cachePost(post)
			}
		}
	}

	posts := make([]model.Post, 0, len(postIDs))
	for _, id := range postIDs {
		if post, found := cachedPosts[model.PostID(id)]; found {
			posts = append(posts, post)
		}
	}

	if err := utils.RenderTemplate(db, w, r, "home", http.StatusOK, posts, valid, username); err != nil {
		log.Error().Err(err).Msg("Error rendering template")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
}

func ShowPost(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	userID, username, valid := userRepo.ValidSession(r, db)

	log := logger.WithRequest(r, userID)
	log.Info().Msg("Showing post detail")

	if r.Method != http.MethodGet {
		log.Warn().Msg("Invalid method for show post")
		utils.RenderError(db, w, r, http.StatusMethodNotAllowed, valid, username)
		return
	}

	id := r.PathValue("id")
	postID, err := strconv.Atoi(id)
	if err != nil {
		log.Warn().Str("post_id", id).Msg("Invalid post ID format")
		utils.RenderError(db, w, r, http.StatusBadRequest, valid, username)
		return
	}

	cacheKey := fmt.Sprintf("post_%d", postID)
	postDetail, found := getPostDetailFromCache(cacheKey)
	if found {
		log.Debug().Int("post_id", postID).Msg("Post detail fetched from cache")
		if err := utils.RenderTemplate(db, w, r, "post", http.StatusOK, postDetail, valid, username); err != nil {
			log.Error().Err(err).Msg("Error rendering template")
			utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		}
		return
	}

	postDetail, statusCode, err := postRepo.FetchPost(db, model.PostID(postID))
	if err != nil {
		log.Error().Err(err).Int("post_id", postID).Msg("Failed to fetch post detail")
		utils.RenderError(db, w, r, statusCode, valid, username)
		return
	}

	cache.AppCache.Set(cacheKey, postDetail, config.CacheTTL)

	if err := utils.RenderTemplate(db, w, r, "post", http.StatusOK, postDetail, valid, username); err != nil {
		log.Error().Err(err).Msg("Error rendering template")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
}

func GetPostCreationForm(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	userID, username, valid := userRepo.ValidSession(r, db)

	log := logger.WithRequest(r, userID)

	if !valid {
		log.Warn().Msg("Unauthorized access to post creation form")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if r.Method != http.MethodGet {
		log.Warn().Msg("Invalid method for post creation form")
		utils.RenderError(db, w, r, http.StatusMethodNotAllowed, valid, username)
		return
	}

	if err := utils.RenderTemplate(db, w, r, "createpost", http.StatusOK, nil, valid, username); err != nil {
		log.Error().Err(err).Msg("Error rendering template")
		utils.RenderError(db, w, r, http.StatusInternalServerError, valid, username)
		return
	}
}

func CreatePost(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	start := time.Now()

	userID, _, valid := userRepo.ValidSession(r, db)
	if !valid {
		w.WriteHeader(401)
		return
	}

	log := logger.WithRequest(r, userID)
	log.Info().Msg("Creating new post")

	if r.Method != http.MethodPost {
		log.Warn().Msg("Invalid method for create post")
		w.WriteHeader(405)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Error().Err(err).Msg("Failed to parse form")
		w.WriteHeader(400)
		return
	}

	title := r.FormValue("title")
	content := r.FormValue("content")
	catids := r.Form["categories"]

	if len(catids) > 0 && strings.Contains(catids[0], ",") {
		catids = strings.Split(catids[0], ",")
	}

	title = html.EscapeString(title)
	content = html.EscapeString(content)

	if len(catids) == 0 || strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
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

	err := categoryRepo.CheckCategories(db, catidsInt)
	if err != nil {
		log.Warn().Ints("category_ids", catidsInt).Msg("Invalid categories")
		w.WriteHeader(400)
		return
	}

	var imagePath string
	imageUrl := r.FormValue("image_url")
	if imageUrl != "" {
		imagePath = imageUrl
	} else {
		file, header, err := r.FormFile("image")
		if err == nil {
			defer file.Close()
			storage := utils.NewLocalStorage(config.BasePath + "web/assets/uploads")
			imagePath, err = storage.Save(file, header)
			if err != nil {
				log.Error().Err(err).Msg("Error saving image")
				w.WriteHeader(500)
				return
			}
		} else if err != http.ErrMissingFile {
			log.Error().Err(err).Msg("Error retrieving image")
			w.WriteHeader(400)
			return
		}
	}

	pid, err := postRepo.StorePost(db, model.AccountID(userID), title, content, imagePath)
	if err != nil {
		log.Error().Err(err).Msg("Error storing post")
		w.WriteHeader(400)
		return
	}

	_, err = postRepo.StoreAllPostCategories(db, pid, catidsInt)
	if err != nil {
		log.Error().Err(err).Int64("post_id", pid).Msg("Failed to store post categories")
		w.WriteHeader(400)
		return
	}

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
	userID, username, valid := userRepo.ValidSession(r, db)
	if !valid {
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
	log := logger.WithRequest(r, userID)

	posts, statusCode, err := postRepo.FetchCreatedPostsByUser(db, model.AccountID(userID), page)
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
	userID, username, valid := userRepo.ValidSession(r, db)
	if !valid {
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
	log := logger.WithRequest(r, userID)

	posts, statusCode, err := postRepo.FetchLikedPostsByUser(db, model.AccountID(userID), page)
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

	userID, _, valid := userRepo.ValidSession(r, db)
	if !valid {
		w.WriteHeader(401)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(400)
		return
	}

	userReaction := r.FormValue("reaction")
	id := r.FormValue("post_id")
	post_id, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	likeCount, dislikeCount, err := postRepo.ReactToPost(db, model.AccountID(userID), model.PostID(post_id), userReaction)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	cache.AppCache.Delete(fmt.Sprintf("post_%d", post_id))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"likesCount": likeCount, "dislikesCount": dislikeCount})
}

func DeletePost(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, _, valid := userRepo.ValidSession(r, db)
	if !valid {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	log := logger.WithRequest(r, userID)

	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		log.Warn().Str("post_id", r.PathValue("id")).Msg("Invalid post ID")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid post ID"})
		return
	}

	statusCode, err := postRepo.DeletePost(db, model.AccountID(userID), model.PostID(postID))
	if err != nil {
		log.Error().Int64("post_id", postID).Err(err).Msg("Failed to delete post")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	cache.AppCache.Delete(fmt.Sprintf("post_%d", postID))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]string{"message": "Post deleted successfully"})
}
