package models

import (
	"context"
	"database/sql"
	"fmt"
	// "forum/server/utils/retry"
	"time"

	"forum/server/database"
)

type Comment struct {
	ID        int
	UserID    int
	PostID    int
	UserName  string
	Content   string
	Likes     int
	Dislikes  int
	CreatedAt string
}

func FetchCommentsByPostID(postID int, db *sql.DB) ([]Comment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	query := `
		SELECT
			c.id,
			c.user_id,
			u.username,
			c.content,
			DATE_FORMAT(c.created_at, '%m/%d/%Y %I:%M %p') AS formatted_created_at,
			(
				SELECT
					COUNT(*)
				FROM
					comment_reactions AS cr
				WHERE
					cr.comment_id = c.id
					AND cr.reaction = 'like'
			) AS likes_count,
			(
				SELECT
					COUNT(*)
				FROM
					comment_reactions AS cr
				WHERE
					cr.comment_id = c.id
					AND cr.reaction = 'dislike'
			) AS dislikes_count
		FROM
			comments c
		INNER JOIN users u 
		ON c.user_id = u.id
		WHERE
			c.post_id = ?
		ORDER BY
			c.created_at DESC
		`

	var comments []Comment

	rows, err := db.QueryContext(ctx, query, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var comment Comment
		err := rows.Scan(
			&comment.ID,
			&comment.UserID,
			&comment.UserName,
			&comment.Content,
			&comment.CreatedAt,
			&comment.Likes,
			&comment.Dislikes,
		)
		if err != nil {
			return nil, err
		}

		comment.PostID = postID

		comments = append(comments, comment)
	}

	return comments, nil
}

func StoreComment(db *sql.DB, user_id, post_id int, content string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO comments (user_id, post_id, content) VALUES (?,?,?)`
	result, err := database.ExecWithMetricsTx(tx, "insert_comment", query, user_id, post_id, content)
	if err != nil {
		return 0, fmt.Errorf("error inserting comment: %v", err)
	}

	commentID, _ := result.LastInsertId()

	var commentCount int
	row, recordError := database.QueryRowWithMetricsAndErrorTx(tx, "count_comments",
		"SELECT COUNT(*) FROM comments WHERE post_id = ?", post_id)
	err = row.Scan(&commentCount)
	recordError(err)
	if err != nil {
		return 0, fmt.Errorf("error counting comments: %v", err)
	}

	query_update_materialized := `UPDATE post_materialized_view SET comment_count = ? WHERE post_id = ?`
	_, err = database.ExecWithMetricsTx(tx, "update_materialized_comment_count", query_update_materialized, commentCount, post_id)
	if err != nil {
		return 0, fmt.Errorf("error updating materialized view: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("error committing transaction: %v", err)
	}

	return commentID, nil
}

func StoreCommentReaction(db *sql.DB, user_id, comment_id int, reaction string) (int64, error) {
	query := `INSERT INTO comment_reactions (user_id,comment_id,reaction) VALUES (?,?,?)`
	result, err := database.ExecWithMetrics(db, "insert_comment_reaction", query, user_id, comment_id, reaction)
	if err != nil {
		fmt.Println(err)
		return 0, fmt.Errorf("error inserting reaction data -> ")
	}
	creactionID, _ := result.LastInsertId()

	return creactionID, nil
}

// Count comments by post ID
func CountCommentsByPostID(db *sql.DB, postID int) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM comments WHERE post_id = ?"
	row, recordError := database.QueryRowWithMetricsAndError(db, "select_comment_count", query, postID)
	err := row.Scan(&count)
	recordError(err)
	if err != nil {
		return 0, fmt.Errorf("error counting comments: %v", err)
	}
	return count, nil
}

// Fetch the creation time of a comment by its ID
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

func ReactToComment(db *sql.DB, user_id, comment_id int, userReaction string) (int, int, error) {
	var likeCount, dislikeCount int
	var dbreaction string
	var err error

	row, recordError := database.QueryRowWithMetricsAndError(db, "select_comment_reaction", "SELECT reaction FROM comment_reactions WHERE user_id=? AND comment_id=?", user_id, comment_id)
	err = row.Scan(&dbreaction)
	if err != sql.ErrNoRows {
		recordError(err)
	}

	if dbreaction == "" {
		_, err = StoreCommentReaction(db, user_id, comment_id, userReaction)
	} else {
		if userReaction == dbreaction {
			query := "DELETE FROM comment_reactions WHERE user_id = ? AND comment_id = ?"
			_, err = database.ExecWithMetrics(db, "delete_comment_reaction", query, user_id, comment_id)

		} else {
			query := "UPDATE comment_reactions SET reaction = ? WHERE user_id = ? AND comment_id = ?"
			_, err = database.ExecWithMetrics(db, "update_comment_reaction", query, userReaction, user_id, comment_id)
		}
	}
	if err != nil {
		return 0, 0, err
	}

	row1, recordError1 := database.QueryRowWithMetricsAndError(db, "select_comment_like_count", "SELECT COUNT(*) FROM comment_reactions WHERE comment_id=? AND reaction=?", comment_id, "like")
	err = row1.Scan(&likeCount)
	recordError1(err)
	if err != nil {
		return 0, 0, fmt.Errorf("error fetching likes count: %v", err)
	}
	row2, recordError2 := database.QueryRowWithMetricsAndError(db, "select_comment_dislike_count", "SELECT COUNT(*) FROM comment_reactions WHERE comment_id=? AND reaction=?", comment_id, "dislike")
	err = row2.Scan(&dislikeCount)
	recordError2(err)
	if err != nil {
		return 0, 0, fmt.Errorf("error fetching likes count: %v", err)
	}
	return likeCount, dislikeCount, nil
}
