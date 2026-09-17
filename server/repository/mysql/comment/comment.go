package comment

import (
	"database/sql"
	"fmt"
	"time"

	"forum/server/database"
	"forum/server/model"

	"github.com/google/uuid"
)

// FetchCommentsByPostID returns the comments of a post, newest first, with the
// like/dislike counts of each comment folded in.
//
// This reads the account-based schema: `comments.user_id` is a CHAR(36) uuid and
// reactions live in the shared `likes` table. The controller still passes an int
// post id, so the list endpoint is what enforces the int64 boundary.
func FetchCommentsByPostID(db *sql.DB, postID int64) ([]model.Comment, error) {
	query := `
		SELECT
			c.id,
			c.user_id,
			a.username,
			c.post_id,
			c.parent_comment_id,
			c.content,
			COALESCE(SUM(l.reaction = 'like'), 0) AS likes,
			COALESCE(SUM(l.reaction = 'dislike'), 0) AS dislikes,
			c.created_at
		FROM comments c
		INNER JOIN accounts a ON a.id = c.user_id
		LEFT JOIN likes l ON l.target_type = 'comment' AND l.target_id = c.id
		WHERE c.post_id = ?
		GROUP BY c.id, c.user_id, a.username, c.post_id, c.parent_comment_id, c.content, c.created_at
		ORDER BY c.created_at DESC`

	rows, err := database.QueryWithMetrics(db, "select_comments", query, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := make([]model.Comment, 0)
	for rows.Next() {
		var (
			comment         model.Comment
			parentCommentID sql.NullInt64
		)
		if err := rows.Scan(
			&comment.ID,
			&comment.UserID,
			&comment.Username,
			&comment.PostID,
			&parentCommentID,
			&comment.Content,
			&comment.LikeCount,
			&comment.DislikeCount,
			&comment.CreatedAt,
		); err != nil {
			return nil, err
		}
		if parentCommentID.Valid {
			parent := model.CommentID(parentCommentID.Int64)
			comment.ParentCommentID = &parent
		}
		comments = append(comments, comment)
	}

	return comments, rows.Err()
}

// CommentListItem is a comment joined with its author and reaction counts, which
// is what the API and the post detail page actually render.
type CommentListItem struct {
	ID        int64
	PostID    int64
	UserID    uuid.UUID
	Username  string
	Content   string
	Likes     int
	Dislikes  int
	CreatedAt time.Time
}

// FetchCommentListItems returns comments of a post with author name and counts.
func FetchCommentListItems(db *sql.DB, postID int64, offset, limit int) ([]CommentListItem, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT
			c.id,
			c.post_id,
			c.user_id,
			a.username,
			c.content,
			c.created_at,
			COALESCE(SUM(l.reaction = 'like'), 0) AS likes,
			COALESCE(SUM(l.reaction = 'dislike'), 0) AS dislikes
		FROM comments c
		INNER JOIN accounts a ON a.id = c.user_id
		LEFT JOIN likes l ON l.target_type = 'comment' AND l.target_id = c.id
		WHERE c.post_id = ?
		GROUP BY c.id, c.post_id, c.user_id, a.username, c.content, c.created_at
		ORDER BY c.created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := database.QueryWithMetrics(db, "select_comment_list", query, postID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]CommentListItem, 0)
	for rows.Next() {
		var item CommentListItem
		if err := rows.Scan(
			&item.ID,
			&item.PostID,
			&item.UserID,
			&item.Username,
			&item.Content,
			&item.CreatedAt,
			&item.Likes,
			&item.Dislikes,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// StoreComment inserts a comment for the given account and returns its id.
// It also inserts the count-affecting row in one transaction so a partial write
// cannot leave the post without its comment.
func StoreComment(db *sql.DB, userID uuid.UUID, postID int64, content string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO comments (user_id, post_id, content) VALUES (?,?,?)`
	result, err := database.ExecWithMetricsTx(tx, "insert_comment", query, userID, postID, content)
	if err != nil {
		return 0, fmt.Errorf("error inserting comment: %v", err)
	}

	commentID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error reading comment id: %v", err)
	}

	queryUpdatePost := `UPDATE posts SET comment_count = comment_count + 1 WHERE id = ?`
	_, err = database.ExecWithMetricsTx(tx, "update_post_comment_count", queryUpdatePost, postID)
	if err != nil {
		return 0, fmt.Errorf("error updating post comment count: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("error committing transaction: %v", err)
	}

	return commentID, nil
}

// CountCommentsByPostID returns how many comments a post has.
func CountCommentsByPostID(db *sql.DB, postID int64) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM comments WHERE post_id = ?`
	row, recordError := database.QueryRowWithMetricsAndError(db, "select_comment_count", query, postID)
	err := row.Scan(&count)
	recordError(err)
	if err != nil {
		return 0, fmt.Errorf("error counting comments: %v", err)
	}
	return count, nil
}

// PostIDForComment returns the post a comment belongs to, which is needed to
// invalidate the right cache after a reaction.
func PostIDForComment(db *sql.DB, commentID int64) (int64, error) {
	var postID int64
	row, recordError := database.QueryRowWithMetricsAndError(
		db, "select_comment_post_id",
		`SELECT post_id FROM comments WHERE id = ?`, commentID,
	)
	err := row.Scan(&postID)
	recordError(err)
	if err != nil {
		return 0, err
	}
	return postID, nil
}

// FetchCommentTimeByID returns a comment's creation time formatted for display.
func FetchCommentTimeByID(db *sql.DB, commentID int64) (string, error) {
	var commentTime string
	query := "SELECT DATE_FORMAT(created_at, '%m/%d/%Y %I:%M %p') AS formatted_created_at FROM comments WHERE id = ?"
	row, recordError := database.QueryRowWithMetricsAndError(db, "select_comment_time", query, commentID)
	err := row.Scan(&commentTime)
	recordError(err)
	if err != nil {
		return "", fmt.Errorf("error fetching comment time: %v", err)
	}
	return commentTime, nil
}

// UpsertCommentReaction applies like/dislike/toggle semantics on the shared
// `likes` table and returns the comment's new counts.
//
// Repeating the same reaction removes it; the opposite reaction replaces it.
func UpsertCommentReaction(db *sql.DB, userID uuid.UUID, commentID int64, reaction string) (int, int, error) {
	var current string
	row, recordError := database.QueryRowWithMetricsAndError(
		db, "select_comment_reaction",
		`SELECT reaction FROM likes WHERE user_id = ? AND target_type = 'comment' AND target_id = ?`,
		userID, commentID,
	)
	err := row.Scan(&current)
	switch {
	case err == sql.ErrNoRows:
		current = ""
	case err != nil:
		recordError(err)
		return 0, 0, err
	}

	switch {
	case current == "":
		_, err = database.ExecWithMetrics(db, "insert_comment_reaction",
			`INSERT INTO likes (user_id, target_type, target_id, reaction) VALUES (?, 'comment', ?, ?)`,
			userID, commentID, reaction)
	case current == reaction:
		_, err = database.ExecWithMetrics(db, "delete_comment_reaction",
			`DELETE FROM likes WHERE user_id = ? AND target_type = 'comment' AND target_id = ?`,
			userID, commentID)
	default:
		_, err = database.ExecWithMetrics(db, "update_comment_reaction",
			`UPDATE likes SET reaction = ? WHERE user_id = ? AND target_type = 'comment' AND target_id = ?`,
			reaction, userID, commentID)
	}
	if err != nil {
		return 0, 0, err
	}

	return CountCommentReactions(db, commentID)
}

// CountCommentReactions returns the like and dislike totals of a comment.
func CountCommentReactions(db *sql.DB, commentID int64) (int, int, error) {
	var likes, dislikes int
	row, recordError := database.QueryRowWithMetricsAndError(
		db, "select_comment_reaction_counts",
		`SELECT
			COALESCE(SUM(reaction = 'like'), 0),
			COALESCE(SUM(reaction = 'dislike'), 0)
		 FROM likes WHERE target_type = 'comment' AND target_id = ?`,
		commentID,
	)
	err := row.Scan(&likes, &dislikes)
	recordError(err)
	if err != nil {
		return 0, 0, fmt.Errorf("error fetching comment reaction counts: %v", err)
	}
	return likes, dislikes, nil
}
