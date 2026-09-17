package post

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"forum/server/database"
	"forum/server/model"
	commentRepo "forum/server/repository/mysql/comment"
	"forum/server/utils/retry"

	"github.com/google/uuid"
)

type PostDetail struct {
	Post     model.Post
	Comments []model.Comment
}

func FetchPosts(db *sql.DB, currentPage int) ([]model.Post, int, error) {
	var posts []model.Post

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `SELECT
		post_id,
		user_id,
		title,
		content,
		created_at
	FROM
		post_materialized_view
	ORDER BY
		created_at DESC
	LIMIT 10 OFFSET ?`

	retryConfig := retry.DatabaseQueryRetryConfig()
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		return database.QueryWithMetrics(db, "select_posts_materialized", query, currentPage)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()

	for rows.Next() {
		var post model.Post
		var postID int64
		var userID uuid.UUID

		err := rows.Scan(
			&postID,
			&userID,
			&post.Title,
			&post.Content,
			&post.CreatedAt,
		)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ID = model.PostID(postID)
		post.UserID = model.AccountID(userID)

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, 500, err
	}

	return posts, 200, nil
}

// Returns posts given their IDs
func FetchPostsByIDs(db *sql.DB, postIDs []int) (map[model.PostID]model.Post, error) {
	if len(postIDs) == 0 {
		return make(map[model.PostID]model.Post), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	placeholders := make([]string, len(postIDs))
	args := make([]interface{}, len(postIDs))
	for i, id := range postIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`SELECT
		post_id,
		user_id,
		title,
		content,
		created_at
	FROM
		post_materialized_view
	WHERE post_id IN (%s)
	ORDER BY created_at DESC`, strings.Join(placeholders, ","))

	retryConfig := retry.DatabaseQueryRetryConfig()
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		return database.QueryWithMetrics(db, "select_posts_by_ids", query, args...)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, err
	}
	defer rows.Close()

	result := make(map[model.PostID]model.Post)
	for rows.Next() {
		var post model.Post
		var postID int64
		var userID uuid.UUID

		err := rows.Scan(&postID, &userID, &post.Title, &post.Content, &post.CreatedAt)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, err
		}
		post.ID = model.PostID(postID)
		post.UserID = model.AccountID(userID)

		result[post.ID] = post
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, err
	}

	return result, nil
}

func FetchPostIDsByTimestamp(db *sql.DB, offset, limit int) ([]int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT post_id, DATE_FORMAT(created_at, '%m/%d/%Y %I:%M %p') AS formatted_created_at
		FROM post_materialized_view
		ORDER BY created_at DESC, post_id DESC
		LIMIT ? OFFSET ?`

	retryConfig := retry.DatabaseQueryRetryConfig()
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		return database.QueryWithMetrics(db, "select_post_ids_page", query, limit, offset)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, "", err
	}
	defer rows.Close()

	var postIDs []int
	var firstTimestamp string

	for rows.Next() {
		var id int
		var timestamp string
		if err := rows.Scan(&id, &timestamp); err != nil {
			log.Println("Error scanning row:", err)
			return nil, "", err
		}

		postIDs = append(postIDs, id)
		if firstTimestamp == "" {
			firstTimestamp = timestamp
		}
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, "", err
	}

	return postIDs, firstTimestamp, nil
}

func FetchPost(db *sql.DB, postID model.PostID) (PostDetail, int, error) {
	var post model.Post
	post.ID = postID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `SELECT
		user_id,
		title,
		content,
		created_at
	FROM
		post_materialized_view
	WHERE post_id = ?`

	retryConfig := retry.DatabaseQueryRetryConfig()
	var userID uuid.UUID
	_, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Row, error) {

		row, recordError := database.QueryRowWithMetricsAndError(db, "select_post_detail_materialized", query, int64(postID))

		err := row.Scan(
			&userID,
			&post.Title,
			&post.Content,
			&post.CreatedAt)
		recordError(err)
		return row, err
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return PostDetail{}, 404, fmt.Errorf("post not found: %w", err)
		}
		log.Println("Error scanning row:", err)
		return PostDetail{}, 500, err
	}
	post.UserID = model.AccountID(userID)

	comments, err := commentRepo.FetchCommentsByPostID(db, int64(postID))
	if err != nil {
		log.Println("Error fetching comments from the database:", err)
	}

	return PostDetail{
		Post:     post,
		Comments: comments,
	}, 200, nil
}

func FetchPostsByCategory(db *sql.DB, categoryID int, currentpage int) ([]model.Post, int, error) {
	var posts []model.Post

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT
			pmv.post_id,
			pmv.user_id,
			pmv.title,
			pmv.content,
			pmv.created_at
		FROM
			post_materialized_view pmv
			INNER JOIN post_category pc ON pmv.post_id = pc.post_id
		WHERE pc.category_id = ?
		ORDER BY
			pmv.created_at DESC
		LIMIT 10 OFFSET ?`
	retryConfig := retry.DatabaseQueryRetryConfig()
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		return database.QueryWithMetrics(db, "select_posts_by_category_materialized", query, categoryID, currentpage)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()
	for rows.Next() {
		var post model.Post
		var postID int64
		var userID uuid.UUID
		err := rows.Scan(&postID,
			&userID,
			&post.Title,
			&post.Content,
			&post.CreatedAt)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ID = model.PostID(postID)
		post.UserID = model.AccountID(userID)

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, 500, err
	}

	return posts, 200, nil
}

func FetchPostIDsForCategoryPage(db *sql.DB, categoryID, offset, limit int) ([]int, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT pmv.post_id, DATE_FORMAT(pmv.created_at, '%m/%d/%Y %I:%M %p') AS formatted_created_at
		FROM post_materialized_view pmv
		INNER JOIN post_category pc ON pmv.post_id = pc.post_id
		WHERE pc.category_id = ?
		ORDER BY pmv.created_at DESC, pmv.post_id DESC
		LIMIT ? OFFSET ?`

	retryConfig := retry.DatabaseQueryRetryConfig()
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		return database.QueryWithMetrics(db, "select_post_ids_category_page", query, categoryID, limit, offset)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, "", err
	}
	defer rows.Close()

	var postIDs []int
	var firstTimestamp string

	for rows.Next() {
		var id int
		var timestamp string
		if err := rows.Scan(&id, &timestamp); err != nil {
			log.Println("Error scanning row:", err)
			return nil, "", err
		}

		postIDs = append(postIDs, id)
		if firstTimestamp == "" {
			firstTimestamp = timestamp
		}
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, "", err
	}

	return postIDs, firstTimestamp, nil
}

func FetchCreatedPostsByUser(db *sql.DB, user_id model.AccountID, currentPage int) ([]model.Post, int, error) {
	var posts []model.Post

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	retryConfig := retry.DatabaseQueryRetryConfig()
	query := `SELECT
		post_id,
		user_id,
		title,
		content,
		created_at
	FROM
		post_materialized_view
	WHERE user_id = ?
	ORDER BY
		created_at DESC
	LIMIT 10 OFFSET ?`
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		return database.QueryWithMetrics(db, "select_posts_by_user_materialized", query, uuid.UUID(user_id), currentPage)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()

	for rows.Next() {
		var post model.Post
		var postID int64
		var userID uuid.UUID
		err := rows.Scan(&postID,
			&userID,
			&post.Title,
			&post.Content,
			&post.CreatedAt)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ID = model.PostID(postID)
		post.UserID = model.AccountID(userID)

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, 500, err
	}

	return posts, 200, nil
}

func FetchLikedPostsByUser(db *sql.DB, user_id model.AccountID, currentPage int) ([]model.Post, int, error) {
	var posts []model.Post

	query := `SELECT
		pmv.post_id,
		pmv.user_id,
		pmv.title,
		pmv.content,
		pmv.created_at
	FROM
		post_materialized_view pmv
		INNER JOIN post_reactions pr ON pmv.post_id = pr.post_id
	WHERE pr.user_id = ? AND pr.reaction = 'like' 
	ORDER BY
		pmv.created_at DESC
	LIMIT 10 OFFSET ?`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	retryConfig := retry.DatabaseQueryRetryConfig()
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		return database.QueryWithMetrics(db, "select_liked_posts_materialized", query, uuid.UUID(user_id), currentPage)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()

	for rows.Next() {
		var post model.Post
		var postID int64
		var userID uuid.UUID
		err := rows.Scan(&postID,
			&userID,
			&post.Title,
			&post.Content,
			&post.CreatedAt)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ID = model.PostID(postID)
		post.UserID = model.AccountID(userID)

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, 500, err
	}

	return posts, 200, nil
}

func StorePost(db *sql.DB, user_id model.AccountID, title, content, imagePath string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("%v", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO posts (user_id, title, content, image_path) VALUES (?,?,?,?)`
	result, err := database.ExecWithMetricsTx(tx, "insert_post", query, uuid.UUID(user_id), title, content, imagePath)
	if err != nil {
		return 0, fmt.Errorf("%v", err)
	}
	postID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%v", err)
	}

	var username string
	err = tx.QueryRow("SELECT username FROM accounts WHERE id = ?", uuid.UUID(user_id)).Scan(&username)
	if err != nil {
		return 0, fmt.Errorf("%v", err)
	}

	query_materialized :=
		`INSERT INTO 
    	post_materialized_view(post_id, user_id, username, title, content, image_path, created_at, like_count, dislike_count, comment_count, categories_str)
		VALUES (?, ?, ?, ?, ?, ?, (SELECT created_at FROM posts p WHERE p.id = ?), 0, 0, 0, '');`

	_, err = database.ExecWithMetricsTx(tx, "insert_post_materialized_view", query_materialized, postID, uuid.UUID(user_id), username, title, content, imagePath, postID)
	if err != nil {
		return 0, fmt.Errorf("%v", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("%v", err)
	}

	return postID, nil
}

func StorePostCategory(db *sql.DB, post_id int64, category_id int) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO post_category (post_id, category_id) VALUES (?,?)`
	result, err := database.ExecWithMetricsTx(tx, "insert_post_category", query, post_id, category_id)
	if err != nil {
		return 0, fmt.Errorf("error inserting post category: %v", err)
	}
	postcatID, _ := result.LastInsertId()

	var categoriesStr sql.NullString
	query_get_categories := `SELECT
		GROUP_CONCAT(c.label)
	FROM
		categories c
	INNER JOIN post_category pc ON c.id = pc.category_id
	WHERE
		pc.post_id = ?`

	row, recordError := database.QueryRowWithMetricsAndErrorTx(tx, "select_post_categories", query_get_categories, post_id)
	err = row.Scan(&categoriesStr)
	recordError(err)
	if err != nil {
		return 0, fmt.Errorf("error getting categories: %v", err)
	}

	query_update_materialized := `UPDATE post_materialized_view SET categories_str = ? WHERE post_id = ?`
	_, err = database.ExecWithMetricsTx(tx, "update_materialized_categories", query_update_materialized, categoriesStr.String, post_id)
	if err != nil {
		return 0, fmt.Errorf("error updating materialized view: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("error committing transaction: %v", err)
	}

	return postcatID, nil
}

func StoreAllPostCategories(db *sql.DB, post_id int64, category_ids []int) (int64, error) {
	if len(category_ids) == 0 {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	var queryBuilder strings.Builder
	queryBuilder.WriteString("INSERT INTO post_category (post_id, category_id) VALUES ")

	values := []interface{}{}
	for i, category_id := range category_ids {
		if i > 0 {
			queryBuilder.WriteString(", ")
		}
		queryBuilder.WriteString("(?, ?)")
		values = append(values, post_id, category_id)
	}

	_, err = database.ExecWithMetricsTx(tx, "insert_post_categories_bulk", queryBuilder.String(), values...)
	if err != nil {
		return 0, fmt.Errorf("error inserting categories: %v", err)
	}

	var categoriesStr sql.NullString
	query_get_categories := `SELECT
		GROUP_CONCAT(c.label)
	FROM
		categories c
	INNER JOIN post_category pc ON c.id = pc.category_id
	WHERE
		pc.post_id = ?`

	row, recordError := database.QueryRowWithMetricsAndErrorTx(tx, "select_post_categories", query_get_categories, post_id)
	err = row.Scan(&categoriesStr)
	recordError(err)
	if err != nil {
		return 0, fmt.Errorf("error getting categories: %v", err)
	}

	query_update_materialized := `UPDATE post_materialized_view SET categories_str = ? WHERE post_id = ?`
	_, err = database.ExecWithMetricsTx(tx, "update_materialized_categories", query_update_materialized, categoriesStr.String, post_id)
	if err != nil {
		return 0, fmt.Errorf("error updating materialized view: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("error committing transaction: %v", err)
	}

	return int64(len(category_ids)), nil
}

func ReactToPost(db *sql.DB, user_id model.AccountID, post_id model.PostID, userReaction string) (int, int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	var likeCount, dislikeCount int
	var dbreaction string

	row, recordError := database.QueryRowWithMetricsAndErrorTx(tx, "select_post_reaction",
		"SELECT reaction FROM post_reactions WHERE user_id=? AND post_id=?", uuid.UUID(user_id), int64(post_id))
	err = row.Scan(&dbreaction)
	if err != sql.ErrNoRows {
		recordError(err)
	}
	if err != nil && err != sql.ErrNoRows {
		return 0, 0, fmt.Errorf("error checking existing reaction: %v", err)
	}

	if dbreaction == "" {
		query := `INSERT INTO post_reactions (user_id, post_id, reaction) VALUES (?,?,?)`
		_, err = database.ExecWithMetricsTx(tx, "insert_post_reaction", query, uuid.UUID(user_id), int64(post_id), userReaction)
		if err != nil {
			return 0, 0, fmt.Errorf("error inserting reaction: %v", err)
		}
	} else {
		if userReaction == dbreaction {
			query := "DELETE FROM post_reactions WHERE user_id = ? AND post_id = ?"
			_, err = database.ExecWithMetricsTx(tx, "delete_post_reaction", query, uuid.UUID(user_id), int64(post_id))
			if err != nil {
				return 0, 0, fmt.Errorf("error deleting reaction: %v", err)
			}
		} else {
			query := "UPDATE post_reactions SET reaction = ? WHERE user_id = ? AND post_id = ?"
			_, err = database.ExecWithMetricsTx(tx, "update_post_reaction", query, userReaction, uuid.UUID(user_id), int64(post_id))
			if err != nil {
				return 0, 0, fmt.Errorf("error updating reaction: %v", err)
			}
		}
	}

	row1, recordError1 := database.QueryRowWithMetricsAndErrorTx(tx, "count_post_likes",
		"SELECT COUNT(*) FROM post_reactions WHERE post_id=? AND reaction=?", int64(post_id), "like")
	err = row1.Scan(&likeCount)
	recordError1(err)
	if err != nil {
		return 0, 0, fmt.Errorf("error counting likes: %v", err)
	}

	row2, recordError2 := database.QueryRowWithMetricsAndErrorTx(tx, "count_post_dislikes",
		"SELECT COUNT(*) FROM post_reactions WHERE post_id=? AND reaction=?", int64(post_id), "dislike")
	err = row2.Scan(&dislikeCount)
	recordError2(err)
	if err != nil {
		return 0, 0, fmt.Errorf("error counting dislikes: %v", err)
	}

	query_update_materialized := `UPDATE post_materialized_view SET like_count = ?, dislike_count = ? WHERE post_id = ?`
	_, err = database.ExecWithMetricsTx(tx, "update_materialized_reactions", query_update_materialized, likeCount, dislikeCount, int64(post_id))
	if err != nil {
		return 0, 0, fmt.Errorf("error updating materialized view: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("error committing transaction: %v", err)
	}

	return likeCount, dislikeCount, nil
}

func DeletePost(db *sql.DB, user_id model.AccountID, post_id model.PostID) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := db.Begin()
	if err != nil {
		return 500, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	var postOwnerID model.AccountID
	row, recordError := database.QueryRowWithMetricsAndErrorTx(tx, "select_post_owner",
		"SELECT user_id FROM posts WHERE id = ?", int64(post_id))
	err = row.Scan(&postOwnerID)
	if err != nil {
		recordError(err)
		if err == sql.ErrNoRows {
			return 404, fmt.Errorf("post not found")
		}
		return 500, fmt.Errorf("error checking post ownership: %v", err)
	}
	recordError(nil)

	if postOwnerID != user_id {
		return 403, fmt.Errorf("user is not authorized to delete this post")
	}

	retryConfig := retry.DatabaseQueryRetryConfig()
	_, err = retry.TryWithResult(ctx, retryConfig, func() (sql.Result, error) {
		return database.ExecWithMetricsTx(tx, "delete_post", "DELETE FROM posts WHERE id = ?", int64(post_id))
	})
	if err != nil {
		return 500, fmt.Errorf("error deleting post: %v", err)
	}

	_, err = retry.TryWithResult(ctx, retryConfig, func() (sql.Result, error) {
		return database.ExecWithMetricsTx(tx, "delete_post_materialized", "DELETE FROM post_materialized_view WHERE post_id = ?", int64(post_id))
	})
	if err != nil {
		return 500, fmt.Errorf("error deleting from materialized view: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return 500, fmt.Errorf("error committing transaction: %v", err)
	}

	return 200, nil
}
