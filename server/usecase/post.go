package usecase

import (
	"database/sql"
	"errors"
	"html"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"forum/server/model"
	categoryRepository "forum/server/repository/postgresql/category"
	postRepository "forum/server/repository/postgresql/post"
)

var (
	ErrPostUnauthorized = errors.New("post action requires authentication")
	ErrPostMethod       = errors.New("method not allowed")
	ErrPostInvalidID    = errors.New("invalid post id")
	ErrPostInvalidData  = errors.New("title, content, and categories are required")
	ErrCategoryInvalid  = errors.New("one or more categories are invalid")
)

type PostUsecase struct {
	db   *sql.DB
	auth *AuthUsecase
}

func NewPostUsecase(db *sql.DB, auth *AuthUsecase) *PostUsecase {
	return &PostUsecase{db: db, auth: auth}
}

type CreatePostInput struct {
	Request    *http.Request
	Title      string
	Content    string
	Categories []string
	ImageURL   string
}

type CreatePostResult struct {
	PostID     int64
	Title      string
	Content    string
	Categories []int
	ImagePath  string
}

// CreatePost validates post data, stores the post and its categories, and invalidates list caches.
// Local file persistence remains a controller/storage concern; ImageURL is the already-uploaded URL.
func (uc *PostUsecase) CreatePost(input CreatePostInput) (*CreatePostResult, error) {
	if input.Request == nil || uc.auth == nil {
		return nil, ErrPostUnauthorized
	}

	session := uc.auth.ValidateSession(input.Request)
	if !session.Valid {
		return nil, ErrPostUnauthorized
	}
	if input.Request.Method != http.MethodPost {
		return nil, ErrPostMethod
	}

	title := html.EscapeString(strings.TrimSpace(input.Title))
	content := html.EscapeString(strings.TrimSpace(input.Content))
	if title == "" || content == "" || len(input.Categories) == 0 {
		return nil, ErrPostInvalidData
	}

	categoryIDs, err := parseCategoryIDs(input.Categories)
	if err != nil {
		return nil, err
	}
	if err := categoryRepository.CheckCategories(uc.db, categoryIDs); err != nil {
		return nil, ErrCategoryInvalid
	}

	postID, err := postRepository.StorePost(uc.db, model.AccountID(session.UserID), title, content, nil)
	if err != nil {
		return nil, err
	}
	if _, err := postRepository.StoreAllPostCategories(uc.db, postID, categoryIDs); err != nil {
		return nil, err
	}

	return &CreatePostResult{
		PostID:     postID,
		Title:      title,
		Content:    content,
		Categories: categoryIDs,
		ImagePath:  input.ImageURL,
	}, nil
}

func parseCategoryIDs(values []string) ([]int, error) {
	if len(values) == 1 && strings.Contains(values[0], ",") {
		values = strings.Split(values[0], ",")
	}

	ids := make([]int, 0, len(values))
	for _, value := range values {
		id, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || id <= 0 {
			return nil, ErrCategoryInvalid
		}
		ids = append(ids, id)
	}
	return ids, nil
}

type ReactToPostInput struct {
	Request  *http.Request
	PostID   int64
	Reaction string
}

type ReactToPostResult struct {
	Likes    int
	Dislikes int
}

func (uc *PostUsecase) ReactToPost(input ReactToPostInput) (*ReactToPostResult, error) {
	if input.Request == nil || uc.auth == nil {
		return nil, ErrPostUnauthorized
	}

	session := uc.auth.ValidateSession(input.Request)
	if !session.Valid {
		return nil, ErrPostUnauthorized
	}
	if input.Request.Method != http.MethodPost {
		return nil, ErrPostMethod
	}
	if input.PostID <= 0 {
		return nil, ErrPostInvalidID
	}
	if input.Reaction != "like" && input.Reaction != "dislike" {
		return nil, ErrReactionInvalid
	}

	likes, dislikes, err := postRepository.ReactToPost(uc.db, model.AccountID(session.UserID), model.PostID(input.PostID), input.Reaction)
	if err != nil {
		return nil, err
	}

	return &ReactToPostResult{Likes: likes, Dislikes: dislikes}, nil
}

type DeletePostInput struct {
	Request *http.Request
	PostID  int64
}

type DeletePostResult struct {
	PostID int64
}

func (uc *PostUsecase) DeletePost(input DeletePostInput) (*DeletePostResult, int, error) {
	if input.Request == nil || uc.auth == nil {
		return nil, http.StatusUnauthorized, ErrPostUnauthorized
	}

	session := uc.auth.ValidateSession(input.Request)
	if !session.Valid {
		return nil, http.StatusUnauthorized, ErrPostUnauthorized
	}
	if input.Request.Method != http.MethodDelete {
		return nil, http.StatusMethodNotAllowed, ErrPostMethod
	}
	if input.PostID <= 0 {
		return nil, http.StatusBadRequest, ErrPostInvalidID
	}

	status, err := postRepository.DeletePost(uc.db, model.AccountID(session.UserID), model.PostID(input.PostID))
	if err != nil {
		return nil, status, err
	}

	return &DeletePostResult{PostID: input.PostID}, http.StatusOK, nil
}

// FetchPost returns a post detail.
func (uc *PostUsecase) FetchPost(postID int64) (postRepository.PostDetail, int, error) {
	if postID <= 0 {
		return postRepository.PostDetail{}, http.StatusBadRequest, ErrPostInvalidID
	}

	detail, status, err := postRepository.FetchPost(uc.db, model.PostID(postID))
	if err != nil {
		return detail, status, err
	}
	return detail, status, nil
}

func (uc *PostUsecase) FetchCreatedPosts(userID model.AccountID, offset int) ([]model.Post, int, error) {
	if userID == model.AccountID(uuid.Nil) {
		return nil, http.StatusUnauthorized, ErrPostUnauthorized
	}
	return postRepository.FetchCreatedPostsByUser(uc.db, userID, normalizeOffset(offset))
}

func (uc *PostUsecase) FetchLikedPosts(userID model.AccountID, offset int) ([]model.Post, int, error) {
	if userID == model.AccountID(uuid.Nil) {
		return nil, http.StatusUnauthorized, ErrPostUnauthorized
	}
	return postRepository.FetchLikedPostsByUser(uc.db, userID, normalizeOffset(offset))
}

func (uc *PostUsecase) ValidateSession(r *http.Request) SessionInfo {
	if uc.auth == nil {
		return SessionInfo{}
	}
	return uc.auth.ValidateSession(r)
}

func (uc *PostUsecase) ListPosts(offset int) ([]model.Post, int, error) {
	return postRepository.FetchPosts(uc.db, normalizeOffset(offset))
}

func (uc *PostUsecase) ListPostsByCategory(categoryID, offset int) ([]model.Post, int, error) {
	if err := categoryRepository.CheckCategories(uc.db, []int{categoryID}); err != nil {
		return nil, http.StatusNotFound, ErrCategoryInvalid
	}
	return postRepository.FetchPostsByCategory(uc.db, categoryID, normalizeOffset(offset))
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
