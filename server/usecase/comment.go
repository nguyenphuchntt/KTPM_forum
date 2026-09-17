package usecase

import (
	"database/sql"
	"errors"
	"html"
	"net/http"
	"strings"
	"time"

	"forum/server/dto/request"
	"forum/server/dto/response"
	commentRepository "forum/server/repository/mysql/comment"
)

var (
	// ErrCommentUnauthorized is returned when the caller has no valid session.
	ErrCommentUnauthorized = NewAppError(http.StatusUnauthorized, CodeUnauthorized, "authentication required")
	// ErrCommentNotFound is returned when the comment id does not exist.
	ErrCommentNotFound = NewAppError(http.StatusNotFound, CodeNotFound, "comment not found")
	// ErrCommentInvalidPost is returned when the post id is missing or not positive.
	ErrCommentInvalidPost = NewAppError(http.StatusBadRequest, CodeValidationError, "invalid post id")
	// ErrCommentEmpty is returned when the comment has no content.
	ErrCommentEmpty = NewAppError(http.StatusBadRequest, CodeValidationError, "comment content is required")
	// ErrCommentTooLong is returned when the comment exceeds the stored limit.
	ErrCommentTooLong = NewAppError(http.StatusBadRequest, CodeValidationError, "comment content is too long")
	// ErrCommentInvalid is returned when the comment id is missing or not positive.
	ErrCommentInvalid = NewAppError(http.StatusBadRequest, CodeValidationError, "invalid comment id")
	// ErrReactionInvalid is returned when the reaction is neither like nor dislike.
	ErrReactionInvalid = NewAppError(http.StatusBadRequest, CodeValidationError, "reaction must be like or dislike")
	// ErrCommentStoreFailed is returned when the comment could not be persisted.
	ErrCommentStoreFailed = NewAppError(http.StatusInternalServerError, CodeInternal, "failed to store comment")
	// ErrCommentLoadFailed is returned when the comment list could not be read.
	ErrCommentLoadFailed = NewAppError(http.StatusInternalServerError, CodeInternal, "failed to load comments")
	// ErrReactionFailed is returned when the reaction could not be applied.
	ErrReactionFailed = NewAppError(http.StatusInternalServerError, CodeInternal, "failed to process reaction")
)

// maxCommentLength matches the limit enforced by the comment form validator.
const maxCommentLength = 1800

// CommentUsecase owns the comment rules: who may write, what is stored, which
// cache entries a mutation invalidates. It never writes to an http.ResponseWriter.
type CommentUsecase struct {
	db   *sql.DB
	auth *AuthUsecase
}

func NewCommentUsecase(db *sql.DB, auth *AuthUsecase) *CommentUsecase {
	return &CommentUsecase{db: db, auth: auth}
}

// CreateComment stores a comment authored by the session user.
//
// The session is read from the request, never from the payload, so a caller
// cannot post as somebody else.
func (uc *CommentUsecase) CreateComment(r *http.Request, req request.CreateCommentRequest) (*response.CreateCommentResponse, error) {
	session, err := uc.auth.RequireSession(r)
	if err != nil {
		return nil, ErrCommentUnauthorized
	}

	if req.PostID <= 0 {
		return nil, ErrCommentInvalidPost
	}

	content := html.EscapeString(strings.TrimSpace(req.Content))
	if content == "" {
		return nil, ErrCommentEmpty
	}
	if len(content) > maxCommentLength {
		return nil, ErrCommentTooLong
	}

	commentID, err := commentRepository.StoreComment(uc.db, session.UserID, req.PostID, content)
	if err != nil {
		return nil, ErrCommentStoreFailed
	}

	// The comment count shown on the post detail changed, so drop the cached
	// post row just like the post usecase does after a mutation.
	invalidatePostCache(req.PostID)

	count, err := commentRepository.CountCommentsByPostID(uc.db, req.PostID)
	if err != nil {
		return nil, ErrCommentLoadFailed
	}

	return &response.CreateCommentResponse{
		Comment: response.CommentResponse{
			ID:        commentID,
			PostID:    req.PostID,
			UserID:    session.UserID.String(),
			Username:  session.Username,
			Content:   content,
			Likes:     0,
			Dislikes:  0,
			CreatedAt: time.Now().UTC(),
		},
		CommentsCount: count,
	}, nil
}

// ListComments returns the comments of a post, newest first.
func (uc *CommentUsecase) ListComments(postID int64) (*response.CommentListResponse, error) {
	if postID <= 0 {
		return nil, ErrCommentInvalidPost
	}

	items, err := commentRepository.FetchCommentListItems(uc.db, postID, 0, 0)
	if err != nil {
		return nil, ErrCommentLoadFailed
	}

	comments := make([]response.CommentResponse, 0, len(items))
	for _, item := range items {
		comments = append(comments, response.CommentResponse{
			ID:        item.ID,
			PostID:    item.PostID,
			UserID:    item.UserID.String(),
			Username:  item.Username,
			Content:   item.Content,
			Likes:     item.Likes,
			Dislikes:  item.Dislikes,
			CreatedAt: item.CreatedAt,
		})
	}

	return &response.CommentListResponse{Data: comments}, nil
}

// ReactToComment applies, replaces or removes the session user's reaction and
// returns the comment's new counts.
func (uc *CommentUsecase) ReactToComment(r *http.Request, req request.CommentReactionRequest) (*response.CommentReactionResponse, error) {
	session, err := uc.auth.RequireSession(r)
	if err != nil {
		return nil, ErrCommentUnauthorized
	}

	if req.CommentID <= 0 {
		return nil, ErrCommentInvalid
	}
	if req.Reaction != "like" && req.Reaction != "dislike" {
		return nil, ErrReactionInvalid
	}

	// The lookup doubles as an existence check: reacting to an unknown comment
	// must fail, not silently insert a dangling likes row. It also gives us the
	// post whose detail view shows the counts, so we can invalidate its cache.
	postID, err := commentRepository.PostIDForComment(uc.db, req.CommentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCommentNotFound
		}
		return nil, ErrReactionFailed
	}

	likes, dislikes, err := commentRepository.UpsertCommentReaction(uc.db, session.UserID, req.CommentID, req.Reaction)
	if err != nil {
		return nil, ErrReactionFailed
	}

	invalidatePostCache(postID)

	return &response.CommentReactionResponse{
		CommentID: int(req.CommentID),
		Likes:     likes,
		Dislikes:  dislikes,
	}, nil
}
