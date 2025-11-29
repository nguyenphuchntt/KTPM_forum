package models

import (
	"context"
	"database/sql"
	"fmt"
	"forum/server/utils/retry"
	"log"
	"strings"
	"time"

	"forum/server/database"
)

type Post struct {
	ID            int
	UserID        int
	UserName      string
	Title         string
	Content       string
	CreatedAt     string
	Likes         int
	Dislikes      int
	Comments      int
	CategoriesStr string
	Categories    []string
	ImagePath     string
}

type PostDetail struct {
	Post     Post
	Comments []Comment
}

func FetchPosts(db *sql.DB, currentPage int) ([]Post, int, error) {
	var posts []Post

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `SELECT
		p.id,
		p.user_id,
		u.username,
		p.title,
		p.content,
		p.image_path,
		DATE_FORMAT(p.created_at, '%m/%d/%Y %I:%M %p') AS formatted_created_at,
		(
			SELECT
				COUNT(*)
			FROM
				post_reactions AS pr
			WHERE
				pr.post_id = p.id
				AND pr.reaction = 'like'
		) AS likes_count,
		(
			SELECT
				COUNT(*)
			FROM
				post_reactions AS pr
			WHERE
				pr.post_id = p.id
				AND pr.reaction = 'dislike'
		) AS dislikes_count,
		(
			SELECT
				COUNT(*)
			FROM
				comments c
			WHERE
				c.post_id = p.id
		) AS comments_count,
		(
			SELECT
				GROUP_CONCAT(c.label)
			FROM
				categories c
			INNER JOIN post_category pc ON c.id = pc.category_id
			WHERE
				pc.post_id = p.id
		) AS categories
	FROM
		posts p
		INNER JOIN users u ON p.user_id = u.id
	ORDER BY
		p.created_at DESC
	LIMIT 10 OFFSET ? ;
	`
	retryConfig := retry.DatabaseQueryRetryConfig()
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		// Query to fetch posts
		return database.QueryWithMetrics(db, "select_posts", query, currentPage)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()

	// Iterate through the rows
	for rows.Next() {
		var post Post
		var imagePath sql.NullString
		// Scan the data into the Post struct
		err := rows.Scan(&post.ID,
			&post.UserID,
			&post.UserName,
			&post.Title,
			&post.Content,
			&imagePath,
			&post.CreatedAt,
			&post.Likes,
			&post.Dislikes,
			&post.Comments,
			&post.CategoriesStr)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ImagePath = imagePath.String
		// it came from the  database as "technology,sports...", so we need to split it
		post.Categories = strings.Split(post.CategoriesStr, ",")

		// Format the created_at field to a more readable format
		// post.CreatedAt = utils.FormatTime(post.CreatedAt)
		// Append the Post struct to the posts slice
		posts = append(posts, post)
	}

	// Check for errors during iteration
	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, 500, err
	}

	return posts, 200, nil
}

func FetchPost(db *sql.DB, postID int) (PostDetail, int, error) {
	var post Post
	post.ID = postID

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `SELECT
		p.user_id,
		u.username,
		p.title,
		p.content,
		p.image_path,
		DATE_FORMAT(p.created_at, '%m/%d/%Y %I:%M %p') AS formatted_created_at,
		(
			SELECT COUNT(*)
			FROM post_reactions AS pr
			WHERE pr.post_id = p.id
			AND pr.reaction = 'like'
		) AS likes_count,
		(
			SELECT COUNT(*)
			FROM post_reactions AS pr
			WHERE pr.post_id = p.id
			AND pr.reaction = 'dislike'
		) AS dislikes_count,
		(
			SELECT COUNT(*)
			FROM comments c
			WHERE c.post_id = p.id
		) AS comments_count,
		(
			SELECT GROUP_CONCAT(c.label)
			FROM categories c
			INNER JOIN post_category pc ON c.id = pc.category_id
			WHERE pc.post_id = p.id
		) AS categories
	FROM
		posts p
		INNER JOIN users u ON p.user_id = u.id
	WHERE p.id = ?`
	
	var imagePath sql.NullString
	retryConfig := retry.DatabaseQueryRetryConfig()
	_, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Row, error) {
		// Query to fetch the post
		// Use QueryRow for a single result
		row, recordError := database.QueryRowWithMetricsAndError(db, "select_post_detail", query, postID)

		// Scan the data into the Post struct
		err := row.Scan(
			&post.UserID,
			&post.UserName,
			&post.Title,
			&post.Content,
			&imagePath,
			&post.CreatedAt,
			&post.Likes,
			&post.Dislikes,
			&post.Comments,
			&post.CategoriesStr)
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
	post.ImagePath = imagePath.String

	// Process categories
	post.Categories = strings.Split(post.CategoriesStr, ",")

	// Format the created_at field
	// post.CreatedAt = post.CreatedAt.Format("01/02/2006 03:04 PM")
	comments, err := FetchCommentsByPostID(postID, db)
	if err != nil {
		log.Println("Error fetching comments from the database:", err)
	}

	return PostDetail{
		Post:     post,
		Comments: comments,
	}, 200, nil
}

func FetchPostsByCategory(db *sql.DB, categoryID int, currentpage int) ([]Post, int, error) {
	var posts []Post

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	query := `
		SELECT
			p.id,
			p.user_id,
			u.username,
			p.title,
			p.content,
			p.image_path,
			DATE_FORMAT(p.created_at, '%m/%d/%Y %I:%M %p') AS formatted_created_at,
			(
				SELECT
					COUNT(*)
				FROM
					post_reactions AS pr
				WHERE
					pr.post_id = p.id
					AND pr.reaction = 'like'
			) AS likes_count,
			(
				SELECT
					COUNT(*)
				FROM
					post_reactions AS pr
				WHERE
					pr.post_id = p.id
					AND pr.reaction = 'dislike'
			) AS dislikes_count,
			(
				SELECT
					COUNT(*)
				FROM
					comments c
				WHERE
					c.post_id = p.id
			) AS comments_count,
			(
				SELECT
					GROUP_CONCAT(c.label)
				FROM
					categories c
				INNER JOIN post_category pc ON c.id = pc.category_id
				WHERE
					pc.post_id = p.id
			) AS categories
		FROM
			posts p
			INNER JOIN users u ON p.user_id = u.id
			INNER JOIN post_category pc ON p.id = pc.post_id
		WHERE pc.category_id = ?
		ORDER BY
			p.created_at
		LIMIT 10 OFFSET ? ;
	`
	retryConfig := retry.DatabaseQueryRetryConfig()
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		return database.QueryWithMetrics(db, "select_posts_by_category", query, categoryID, currentpage)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()
	for rows.Next() {
		var post Post
		var imagePath sql.NullString
		err := rows.Scan(&post.ID,
			&post.UserID,
			&post.UserName,
			&post.Title,
			&post.Content,
			&imagePath,
			&post.CreatedAt,
			&post.Likes,
			&post.Dislikes,
			&post.Comments,
			&post.CategoriesStr)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ImagePath = imagePath.String

		// it came from the  database as "technology,sports...", so we need to split it
		post.Categories = strings.Split(post.CategoriesStr, ",")

		// post.CreatedAt = utils.FormatTime(post.CreatedAt)

		posts = append(posts, post)
	}

	// Check for errors during iteration
	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, 500, err
	}

	return posts, 200, nil
}

func FetchCreatedPostsByUser(db *sql.DB, user_id int, currentPage int) ([]Post, int, error) {
	var posts []Post

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	retryConfig := retry.DatabaseQueryRetryConfig()
	// Query to fetch posts
	query := `SELECT
		p.id,
		p.user_id,
		u.username,
		p.title,
		p.content,
		p.image_path,
		DATE_FORMAT(p.created_at, '%m/%d/%Y %I:%M %p') AS formatted_created_at,
		(
			SELECT
				COUNT(*)
			FROM
				post_reactions AS pr
			WHERE
				pr.post_id = p.id
				AND pr.reaction = 'like'
			) AS likes_count,
		(
			SELECT
				COUNT(*)
			FROM
				post_reactions AS pr
			WHERE
				pr.post_id = p.id
				AND pr.reaction = 'dislike'
		) AS dislikes_count,
		(
			SELECT
				COUNT(*)
			FROM
				comments c
			WHERE
				c.post_id = p.id
		) AS comments_count,
		(
			SELECT
				GROUP_CONCAT(c.label)
			FROM
				categories c
			INNER JOIN post_category pc ON c.id = pc.category_id
			WHERE
				pc.post_id = p.id
		) AS categories
	FROM
		posts p
		INNER JOIN users u ON p.user_id = u.id
	WHERE p.user_id = ?
	ORDER BY
		p.created_at DESC
	LIMIT 10 OFFSET ? ;
	`
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		return database.QueryWithMetrics(db, "select_posts_by_user", query, user_id, currentPage)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()

	// Iterate through the rows
	for rows.Next() {
		var post Post
		var imagePath sql.NullString
		// Scan the data into the Post struct
		err := rows.Scan(&post.ID,
			&post.UserID,
			&post.UserName,
			&post.Title,
			&post.Content,
			&imagePath,
			&post.CreatedAt,
			&post.Likes,
			&post.Dislikes,
			&post.Comments,
			&post.CategoriesStr)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ImagePath = imagePath.String
		// it came from the  database as "technology,sports...", so we need to split it
		post.Categories = strings.Split(post.CategoriesStr, ",")

		// Format the created_at field to a more readable format
		// post.CreatedAt = utils.FormatTime(post.CreatedAt)

		// Append the Post struct to the posts slice
		posts = append(posts, post)
	}

	// Check for errors during iteration
	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, 500, err
	}

	return posts, 200, nil
}

func FetchLikedPostsByUser(db *sql.DB, user_id int, currentPage int) ([]Post, int, error) {
	var posts []Post

	// Query to fetch posts
	query := `SELECT
		p.id,
		p.user_id,
		u.username,
		p.title,
		p.content,
		p.image_path,
		DATE_FORMAT(p.created_at, '%m/%d/%Y %I:%M %p') AS formatted_created_at,
		(
			SELECT
				COUNT(*)
			FROM
				post_reactions AS pr
			WHERE
				pr.post_id = p.id
				AND pr.reaction = 'like'
		) AS likes_count,
		(
			SELECT
				COUNT(*)
			FROM
				post_reactions AS pr
			WHERE
				pr.post_id = p.id
				AND pr.reaction = 'dislike'
		) AS dislikes_count,
		(
			SELECT
				COUNT(*)
			FROM
				comments c
			WHERE
				c.post_id = p.id
		) AS comments_count,
		(
			SELECT
				GROUP_CONCAT(c.label)
			FROM
				categories c
			INNER JOIN post_category pc ON c.id = pc.category_id
			WHERE
				pc.post_id = p.id
		) AS categories
	FROM
		posts p
		INNER JOIN users u ON p.user_id = u.id
		INNER JOIN post_reactions pr ON p.id = pr.post_id
	WHERE pr.user_id = ? AND pr.reaction = 'like' 
	ORDER BY
		p.created_at DESC
	LIMIT 10 OFFSET ? ;
	`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	retryConfig := retry.DatabaseQueryRetryConfig()
	rows, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.Rows, error) {
		return database.QueryWithMetrics(db, "select_liked_posts", query, user_id, currentPage)
	})
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()

	// Iterate through the rows
	for rows.Next() {
		var post Post
		var imagePath sql.NullString
		// Scan the data into the Post struct
		err := rows.Scan(&post.ID,
			&post.UserID,
			&post.UserName,
			&post.Title,
			&post.Content,
			&imagePath,
			&post.CreatedAt,
			&post.Likes,
			&post.Dislikes,
			&post.Comments,
			&post.CategoriesStr)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ImagePath = imagePath.String
		// it came from the  database as "technology,sports...", so we need to split it
		post.Categories = strings.Split(post.CategoriesStr, ",")

		// Format the created_at field to a more readable format
		// post.CreatedAt = utils.FormatTime(post.CreatedAt)

		// Append the Post struct to the posts slice
		posts = append(posts, post)
	}

	// Check for errors during iteration
	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, 500, err
	}

	return posts, 200, nil
}

func StorePost(db *sql.DB, user_id int, title, content, imagePath string) (int64, error) {
	query := `INSERT INTO posts (user_id,title,content,image_path) VALUES (?,?,?,?)`

	result, err := database.ExecWithMetrics(db, "insert_post", query, user_id, title, content, imagePath)
	if err != nil {
		return 0, fmt.Errorf("%v", err)
	}

	postID, _ := result.LastInsertId()

	return postID, nil
}

func StorePostCategory(db *sql.DB, post_id int64, category_id int) (int64, error) {
	query := `INSERT INTO post_category (post_id, category_id) VALUES (?,?)`

	result, err := database.ExecWithMetrics(db, "insert_post_category", query, post_id, category_id)
	if err != nil {
		return 0, fmt.Errorf("%v", err)
	}

	postcatID, _ := result.LastInsertId()

	return postcatID, nil
}

func StoreAllPostCategories(db *sql.DB, post_id int64, category_ids []int) (int64, error) {
	if len(category_ids) == 0 {
		return 0, nil
	}

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

	_, err := database.ExecWithMetrics(db, "insert_post_categories", queryBuilder.String(), values...)
	if err != nil {
		return 0, fmt.Errorf("%v", err)
	}

	return int64(len(category_ids)), nil
}

func StorePostReaction(db *sql.DB, user_id, post_id int, reaction string) (int64, error) {
	query := `INSERT INTO post_reactions (user_id,post_id,reaction) VALUES (?,?,?)`
	result, err := database.ExecWithMetrics(db, "insert_post_reaction", query, user_id, post_id, reaction)
	if err != nil {
		return 0, fmt.Errorf("error inserting reaction data -> ")
	}
	preactionID, _ := result.LastInsertId()

	return preactionID, nil
}

func ReactToPost(db *sql.DB, user_id, post_id int, userReaction string) (int, int, error) {
	var likeCount, dislikeCount int
	var dbreaction string
	var err error
	row, recordError := database.QueryRowWithMetricsAndError(db, "select_post_reaction", "SELECT reaction FROM post_reactions WHERE user_id=? AND post_id=?", user_id, post_id)
	err = row.Scan(&dbreaction)
	if err != sql.ErrNoRows {
		recordError(err)
	}

	if dbreaction == "" {
		_, err = StorePostReaction(db, user_id, post_id, userReaction)
	} else {
		if userReaction == dbreaction {
			query := "DELETE FROM post_reactions WHERE user_id = ? AND post_id = ?"
			_, err = database.ExecWithMetrics(db, "delete_post_reaction", query, user_id, post_id)
		} else {
			query := "UPDATE post_reactions SET reaction = ? WHERE user_id = ? AND post_id = ?"
			_, err = database.ExecWithMetrics(db, "update_post_reaction", query, userReaction, user_id, post_id)
		}
	}

	if err != nil {
		return 0, 0, err
	}

	// Fetch the new count of reactions for this post
	row1, recordError1 := database.QueryRowWithMetricsAndError(db, "select_post_like_count", "SELECT COUNT(*) FROM post_reactions WHERE post_id=? AND reaction=?", post_id, "like")
	row1.Scan(&likeCount)
	recordError1(nil)
	row2, recordError2 := database.QueryRowWithMetricsAndError(db, "select_post_dislike_count", "SELECT COUNT(*) FROM post_reactions WHERE post_id=? AND reaction=?", post_id, "dislike")
	row2.Scan(&dislikeCount)
	recordError2(nil)

	return likeCount, dislikeCount, nil
}
