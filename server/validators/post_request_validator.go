package validators

import (
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"

	"forum/server/dto/request"
)

// validates a request for posts index.
// returns:
// - int: HTTP status code.
// - string: Error or success message.
// - int: page number.
func IndexPostsRequest(r *http.Request) (int, string, int) {
	if r.URL.Path != "/" {
		return http.StatusNotFound, "Invalid path", 0
	}

	if r.Method != http.MethodGet {
		return http.StatusMethodNotAllowed, "Invalid HTTP method", 0
	}

	err := r.ParseForm()
	if err != nil {
		return http.StatusBadRequest, "Failed to parse form data", 0
	}

	pageStr := r.FormValue("PageID")
	page, err := strconv.Atoi(pageStr)
	if err != nil && pageStr != "" {
		return http.StatusBadRequest, "PageID must be a valid integer", 0
	}

	if page < 0 {
		return http.StatusBadRequest, "PageID cannot be negative", 0
	}

	return http.StatusOK, "success", page
}

// validates a request for posts by category.
// Returns:
// - int: HTTP status code.
// - string: Error or success message.
// - int: category ID.
// - int: page number.
func IndexPostsByCategoryRequest(r *http.Request) (int, string, int, int) {
	if r.URL.Path != "/" {
		return http.StatusNotFound, "Invalid path", 0, 0
	}

	if r.Method != http.MethodGet {
		return http.StatusMethodNotAllowed, "Invalid HTTP method", 0, 0
	}

	err := r.ParseForm()
	if err != nil {
		return http.StatusBadRequest, "Failed to parse form data", 0, 0
	}

	categorieIdStr := r.PathValue("id")
	categorieId, err := strconv.Atoi(categorieIdStr)
	if err != nil {
		return http.StatusBadRequest, "Category ID must be a valid integer", 0, 0
	}

	pageStr := r.FormValue("PageID")
	page, err := strconv.Atoi(pageStr)
	if err != nil && pageStr != "" {
		return http.StatusBadRequest, "PageID must be a valid integer", 0, 0
	}

	if page <= 0 {
		return http.StatusBadRequest, "PageID must be greater than 0", 0, 0
	}

	return http.StatusOK, "success", categorieId, page
}

// validates a request to show a specific post.
// Returns:
// - int: HTTP status code.
// - string: Error or success message.
// - int: post ID.
func ShowPostRequest(r *http.Request) (int, string, int) {
	if r.Method != http.MethodGet {
		return http.StatusMethodNotAllowed, "Invalid HTTP method", 0
	}

	err := r.ParseForm()
	if err != nil {
		return http.StatusBadRequest, "Failed to parse form data", 0
	}

	postIdStr := r.PathValue("id")
	postId, err := strconv.Atoi(postIdStr)
	if err != nil {
		return http.StatusBadRequest, "Post ID must be a valid integer", 0
	}

	if postId <= 0 {
		return http.StatusBadRequest, "Post ID must be greater than 0", 0
	}

	return http.StatusOK, "success", postId
}

// validates a request to create a new post.
// Returns:
// - int: HTTP status code.
// - string: Error or success message.
// - string: title of the post.
// - string: content of the post.
// - []int: List of category IDs.
func CreatePostRequest(r *http.Request) (int, string, string, string, []int) {
	if r.Method != http.MethodPost {
		return http.StatusMethodNotAllowed, "Invalid HTTP method", "", "", nil
	}

	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		return http.StatusUnsupportedMediaType, "Unsupported content type", "", "", nil
	}

	err := r.ParseForm()
	if err != nil {
		return http.StatusBadRequest, "Failed to parse form data", "", "", nil
	}

	title := r.FormValue("title")
	content := r.FormValue("content")
	categories := r.Form["categories"]

	if strings.TrimSpace(title) == "" {
		return http.StatusBadRequest, "Title is required", "", "", nil
	}
	if len(title) > 100 {
		return http.StatusBadRequest, "Title must not exceed 100 characters", "", "", nil
	}

	if len(categories) == 0 {
		return http.StatusBadRequest, "At least one category is required", "", "", nil
	}

	convertCategories := make([]int, 0, len(categories))
	for _, cat := range categories {
		if cat == "" {
			return http.StatusBadRequest, "Category ID cannot be empty", "", "", nil
		}

		categoryID, err := strconv.Atoi(cat)
		if err != nil {
			return http.StatusBadRequest, "Category ID must be a valid integer", "", "", nil
		}

		convertCategories = append(convertCategories, categoryID)
	}

	if strings.TrimSpace(content) == "" {
		return http.StatusBadRequest, "Content is required", "", "", nil
	}
	if len(content) > 3000 {
		return http.StatusBadRequest, "Content must not exceed 3000 characters", "", "", nil
	}

	return http.StatusOK, "success",
		html.EscapeString(title),
		html.EscapeString(content),
		convertCategories
}

func BindCreatePostRequest(r *http.Request) (request.CreatePostRequest, error) {
	var req request.CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

func ValidateCreatePostRequest(req request.CreatePostRequest) map[string]string {
	details := make(map[string]string)
	if strings.TrimSpace(req.Title) == "" {
		details["title"] = "title is required"
	}
	if strings.TrimSpace(req.Content) == "" {
		details["content"] = "content is required"
	}
	if len(req.Categories) == 0 {
		details["categories"] = "at least one category is required"
	}
	if len(details) > 0 {
		return details
	}
	return nil
}

func BindReactToPostRequest(r *http.Request) (request.ReactToPostRequest, error) {
	var req request.ReactToPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

func ValidateReactToPostRequest(req request.ReactToPostRequest) map[string]string {
	if req.Reaction != "like" && req.Reaction != "dislike" {
		return map[string]string{"reaction": "reaction must be 'like' or 'dislike'"}
	}
	return nil
}
