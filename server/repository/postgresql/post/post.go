package post

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"forum/server/database"
	"forum/server/model"
	commentRepo "forum/server/repository/postgresql/comment"

	"github.com/google/uuid"
)

type PostDetail struct {
	Post     model.Post
	Comments []model.Comment
}

func FetchPosts(db *sql.DB, currentPage int) ([]model.Post, int, error) {
	var posts []model.Post

	query := `SELECT
		p.id,
		p.user_id,
		a.username,
		p.title,
		p.content,
		p.media_id,
		p.like_count,
		p.dislike_count,
		p.comment_count,
		p.created_at
	FROM
		posts p
		INNER JOIN accounts a ON a.id = p.user_id
	ORDER BY
		p.created_at DESC
	LIMIT 10 OFFSET ?`

	rows, err := database.QueryWithMetrics(db, "select_posts", query, currentPage)
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()

	for rows.Next() {
		var post model.Post
		var postID int64
		var userID uuid.UUID
		var mediaID sql.NullInt64

		err := rows.Scan(
			&postID,
			&userID,
			&post.Username,
			&post.Title,
			&post.Content,
			&mediaID,
			&post.LikeCount,
			&post.DislikeCount,
			&post.CommentCount,
			&post.CreatedAt,
		)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ID = model.PostID(postID)
		post.UserID = model.AccountID(userID)
		if mediaID.Valid {
			mID := model.MediaID(mediaID.Int64)
			post.MediaID = &mID
		}

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

	placeholders := make([]string, len(postIDs))
	args := make([]interface{}, len(postIDs))
	for i, id := range postIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`SELECT
		p.id,
		p.user_id,
		a.username,
		p.title,
		p.content,
		p.media_id,
		p.like_count,
		p.dislike_count,
		p.comment_count,
		p.created_at
	FROM
		posts p
		INNER JOIN accounts a ON a.id = p.user_id
	WHERE p.id IN (%s)
	ORDER BY p.created_at DESC`, strings.Join(placeholders, ","))

	rows, err := database.QueryWithMetrics(db, "select_posts_by_ids", query, args...)
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
		var mediaID sql.NullInt64

		err := rows.Scan(
			&postID,
			&userID,
			&post.Username,
			&post.Title,
			&post.Content,
			&mediaID,
			&post.LikeCount,
			&post.DislikeCount,
			&post.CommentCount,
			&post.CreatedAt,
		)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, err
		}
		post.ID = model.PostID(postID)
		post.UserID = model.AccountID(userID)
		if mediaID.Valid {
			mID := model.MediaID(mediaID.Int64)
			post.MediaID = &mID
		}

		result[post.ID] = post
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, err
	}

	return result, nil
}

func FetchPostIDsByTimestamp(db *sql.DB, offset, limit int) ([]int, string, error) {
	query := `
		SELECT id, TO_CHAR(created_at, 'MM/DD/YYYY HH12:MI AM') AS formatted_created_at
		FROM posts
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?`

	rows, err := database.QueryWithMetrics(db, "select_post_ids_page", query, limit, offset)
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

	query := `SELECT
		p.user_id,
		a.username,
		p.title,
		p.content,
		p.media_id,
		p.like_count,
		p.dislike_count,
		p.comment_count,
		p.created_at
	FROM
		posts p
		INNER JOIN accounts a ON a.id = p.user_id
	WHERE p.id = ?`

	var userID uuid.UUID
	var mediaID sql.NullInt64

	row, recordError := database.QueryRowWithMetricsAndError(db, "select_post_detail", query, int64(postID))

	err := row.Scan(
		&userID,
		&post.Username,
		&post.Title,
		&post.Content,
		&mediaID,
		&post.LikeCount,
		&post.DislikeCount,
		&post.CommentCount,
		&post.CreatedAt)
	recordError(err)
	if err != nil {
		if err == sql.ErrNoRows {
			return PostDetail{}, 404, fmt.Errorf("post not found: %w", err)
		}
		log.Println("Error scanning row:", err)
		return PostDetail{}, 500, err
	}
	post.UserID = model.AccountID(userID)
	if mediaID.Valid {
		mID := model.MediaID(mediaID.Int64)
		post.MediaID = &mID
	}

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

	query := `
		SELECT
			p.id,
			p.user_id,
			a.username,
			p.title,
			p.content,
			p.media_id,
			p.like_count,
			p.dislike_count,
			p.comment_count,
			p.created_at
		FROM
			posts p
			INNER JOIN accounts a ON a.id = p.user_id
			INNER JOIN post_category pc ON p.id = pc.post_id
		WHERE pc.category_id = ?
		ORDER BY
			p.created_at DESC
		LIMIT 10 OFFSET ?`
	rows, err := database.QueryWithMetrics(db, "select_posts_by_category", query, categoryID, currentpage)
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()
	for rows.Next() {
		var post model.Post
		var postID int64
		var userID uuid.UUID
		var mediaID sql.NullInt64
		err := rows.Scan(
			&postID,
			&userID,
			&post.Username,
			&post.Title,
			&post.Content,
			&mediaID,
			&post.LikeCount,
			&post.DislikeCount,
			&post.CommentCount,
			&post.CreatedAt,
		)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ID = model.PostID(postID)
		post.UserID = model.AccountID(userID)
		if mediaID.Valid {
			mID := model.MediaID(mediaID.Int64)
			post.MediaID = &mID
		}

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, 500, err
	}

	return posts, 200, nil
}

func FetchPostIDsForCategoryPage(db *sql.DB, categoryID, offset, limit int) ([]int, string, error) {
	query := `
		SELECT p.id, TO_CHAR(p.created_at, 'MM/DD/YYYY HH12:MI AM') AS formatted_created_at
		FROM posts p
		INNER JOIN post_category pc ON p.id = pc.post_id
		WHERE pc.category_id = ?
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT ? OFFSET ?`

	rows, err := database.QueryWithMetrics(db, "select_post_ids_category_page", query, categoryID, limit, offset)
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

	query := `SELECT
		p.id,
		p.user_id,
		a.username,
		p.title,
		p.content,
		p.media_id,
		p.like_count,
		p.dislike_count,
		p.comment_count,
		p.created_at
	FROM
		posts p
		INNER JOIN accounts a ON a.id = p.user_id
	WHERE p.user_id = ?
	ORDER BY
		p.created_at DESC
	LIMIT 10 OFFSET ?`
	rows, err := database.QueryWithMetrics(db, "select_posts_by_user", query, uuid.UUID(user_id), currentPage)
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()

	for rows.Next() {
		var post model.Post
		var postID int64
		var userID uuid.UUID
		var mediaID sql.NullInt64
		err := rows.Scan(
			&postID,
			&userID,
			&post.Username,
			&post.Title,
			&post.Content,
			&mediaID,
			&post.LikeCount,
			&post.DislikeCount,
			&post.CommentCount,
			&post.CreatedAt,
		)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ID = model.PostID(postID)
		post.UserID = model.AccountID(userID)
		if mediaID.Valid {
			mID := model.MediaID(mediaID.Int64)
			post.MediaID = &mID
		}

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
		p.id,
		p.user_id,
		a.username,
		p.title,
		p.content,
		p.media_id,
		p.like_count,
		p.dislike_count,
		p.comment_count,
		p.created_at
	FROM
		posts p
		INNER JOIN accounts a ON a.id = p.user_id
		INNER JOIN likes l ON l.target_type = 'post' AND l.target_id = p.id
	WHERE l.user_id = ? AND l.reaction = 'like' 
	ORDER BY
		p.created_at DESC
	LIMIT 10 OFFSET ?`
	rows, err := database.QueryWithMetrics(db, "select_liked_posts", query, uuid.UUID(user_id), currentPage)
	if err != nil {
		log.Println("Error executing query:", err)
		return nil, 500, err
	}
	defer rows.Close()

	for rows.Next() {
		var post model.Post
		var postID int64
		var userID uuid.UUID
		var mediaID sql.NullInt64
		err := rows.Scan(
			&postID,
			&userID,
			&post.Username,
			&post.Title,
			&post.Content,
			&mediaID,
			&post.LikeCount,
			&post.DislikeCount,
			&post.CommentCount,
			&post.CreatedAt,
		)
		if err != nil {
			log.Println("Error scanning row:", err)
			return nil, 500, err
		}
		post.ID = model.PostID(postID)
		post.UserID = model.AccountID(userID)
		if mediaID.Valid {
			mID := model.MediaID(mediaID.Int64)
			post.MediaID = &mID
		}

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return nil, 500, err
	}

	return posts, 200, nil
}

// PostSearchResult is one full-text-search hit: the post, its relevance rank and
// the matching terms marked up in <mark> — once inside the title and once inside
// an excerpt of the content.
type PostSearchResult struct {
	Post model.Post
	Rank float32
	// TitleSnippet is the post title with the query terms marked. A term that
	// matched only the title has no other place to show up, so highlighting the
	// title is what makes such a hit legible.
	TitleSnippet string
	Snippet      string
}

type SearchPostsPage struct {
	Results []PostSearchResult
	// Total matching posts, independent of LIMIT/OFFSET, so the caller can
	// report how many results exist beyond this page.
	TotalCount int
}

// SearchPosts runs a full-text search over posts. Both the title and the content
// are searched; the search_vector column weights a title hit above a content hit,
// and the weights are spelled out in ts_rank so that ordering is visible here
// rather than implied by the A/B labels over in the migration. The array is
// {D, C, B, A} — the PostgreSQL default, with A = title at 1.0 and B = content at
// 0.4, so a title match outranks a content match by 2.5x.
//
// query goes through websearch_to_tsquery, which parses natural input ("exact
// phrase", OR, -exclude) and never raises a syntax error — an unusable query
// simply yields no rows.
//
// Both snippets are built from stored text, which the write paths already ran
// through html.EscapeString, so the only real markup they can contain is the
// <mark> pair ts_headline inserts. Keep that escaping in place: the templates
// render these fields unescaped on purpose.
func SearchPosts(db *sql.DB, query string, offset int) (SearchPostsPage, int, error) {
	var page SearchPostsPage

	sqlQuery := `SELECT
		p.id,
		p.user_id,
		a.username,
		p.title,
		p.content,
		p.media_id,
		p.like_count,
		p.dislike_count,
		p.comment_count,
		p.created_at,
		ts_rank('{0.1, 0.2, 0.4, 1.0}', p.search_vector, q.query) AS rank,
		ts_headline('english', p.title, q.query,
			'StartSel=<mark>, StopSel=</mark>, MaxWords=100, MinWords=1') AS title_snippet,
		ts_headline('english', p.content, q.query,
			'StartSel=<mark>, StopSel=</mark>, MaxWords=35, MinWords=15') AS snippet,
		COUNT(*) OVER() AS total_count
	FROM
		posts p
		INNER JOIN accounts a ON a.id = p.user_id
		CROSS JOIN websearch_to_tsquery('english', ?) AS q(query)
	WHERE p.search_vector @@ q.query
	ORDER BY
		rank DESC,
		p.created_at DESC
	LIMIT 10 OFFSET ?`

	rows, err := database.QueryWithMetrics(db, "search_posts", sqlQuery, query, offset)
	if err != nil {
		log.Println("Error executing query:", err)
		return page, 500, err
	}
	defer rows.Close()

	for rows.Next() {
		var result PostSearchResult
		var postID int64
		var userID uuid.UUID
		var mediaID sql.NullInt64
		err := rows.Scan(
			&postID,
			&userID,
			&result.Post.Username,
			&result.Post.Title,
			&result.Post.Content,
			&mediaID,
			&result.Post.LikeCount,
			&result.Post.DislikeCount,
			&result.Post.CommentCount,
			&result.Post.CreatedAt,
			&result.Rank,
			&result.TitleSnippet,
			&result.Snippet,
			&page.TotalCount,
		)
		if err != nil {
			log.Println("Error scanning row:", err)
			return page, 500, err
		}
		result.Post.ID = model.PostID(postID)
		result.Post.UserID = model.AccountID(userID)
		if mediaID.Valid {
			mID := model.MediaID(mediaID.Int64)
			result.Post.MediaID = &mID
		}

		page.Results = append(page.Results, result)
	}

	if err = rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
		return page, 500, err
	}

	return page, 200, nil
}

func StorePost(db *sql.DB, user_id model.AccountID, title, content string, mediaID *model.MediaID) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("%v", err)
	}
	defer tx.Rollback()

	var mediaIDVal interface{}
	if mediaID != nil {
		mediaIDVal = int64(*mediaID)
	} else {
		mediaIDVal = nil
	}

	query := `INSERT INTO posts (user_id, title, content, media_id) VALUES (?,?,?,?) RETURNING id`
	var postID int64
	row, recordError := database.QueryRowWithMetricsAndErrorTx(tx, "insert_post", query, uuid.UUID(user_id), title, content, mediaIDVal)
	if err := row.Scan(&postID); err != nil {
		recordError(err)
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

	query := `INSERT INTO post_category (post_id, category_id) VALUES (?,?) RETURNING id`
	var postcatID int64
	row, recordError := database.QueryRowWithMetricsAndErrorTx(tx, "insert_post_category", query, post_id, category_id)
	if err := row.Scan(&postcatID); err != nil {
		recordError(err)
		return 0, fmt.Errorf("error inserting post category: %v", err)
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

	var dbreaction string
	row, recordError := database.QueryRowWithMetricsAndErrorTx(tx, "select_post_reaction",
		"SELECT reaction FROM likes WHERE user_id=? AND target_type='post' AND target_id=?", uuid.UUID(user_id), int64(post_id))
	err = row.Scan(&dbreaction)
	if err != nil && err != sql.ErrNoRows {
		recordError(err)
		return 0, 0, fmt.Errorf("error checking existing reaction: %v", err)
	}

	var likeDelta, dislikeDelta int
	if dbreaction == "" {
		query := `INSERT INTO likes (user_id, target_type, target_id, reaction) VALUES (?,'post',?,?)`
		_, err = database.ExecWithMetricsTx(tx, "insert_post_reaction", query, uuid.UUID(user_id), int64(post_id), userReaction)
		if err != nil {
			return 0, 0, fmt.Errorf("error inserting reaction: %v", err)
		}
		if userReaction == "like" {
			likeDelta = 1
		} else if userReaction == "dislike" {
			dislikeDelta = 1
		}
	} else {
		if userReaction == dbreaction {
			query := "DELETE FROM likes WHERE user_id = ? AND target_type = 'post' AND target_id = ?"
			_, err = database.ExecWithMetricsTx(tx, "delete_post_reaction", query, uuid.UUID(user_id), int64(post_id))
			if err != nil {
				return 0, 0, fmt.Errorf("error deleting reaction: %v", err)
			}
			if userReaction == "like" {
				likeDelta = -1
			} else if userReaction == "dislike" {
				dislikeDelta = -1
			}
		} else {
			query := "UPDATE likes SET reaction = ? WHERE user_id = ? AND target_type = 'post' AND target_id = ?"
			_, err = database.ExecWithMetricsTx(tx, "update_post_reaction", query, userReaction, uuid.UUID(user_id), int64(post_id))
			if err != nil {
				return 0, 0, fmt.Errorf("error updating reaction: %v", err)
			}
			if userReaction == "like" {
				likeDelta = 1
				dislikeDelta = -1
			} else {
				likeDelta = -1
				dislikeDelta = 1
			}
		}
	}

	queryUpdateCount := `UPDATE posts SET 
		like_count = GREATEST(0, like_count + ?), 
		dislike_count = GREATEST(0, dislike_count + ?) 
		WHERE id = ?`
	_, err = database.ExecWithMetricsTx(tx, "update_post_counts", queryUpdateCount, likeDelta, dislikeDelta, int64(post_id))
	if err != nil {
		return 0, 0, fmt.Errorf("error updating post counts: %v", err)
	}

	var likeCount, dislikeCount int
	rowCounts, recordError2 := database.QueryRowWithMetricsAndErrorTx(tx, "select_post_counts",
		"SELECT like_count, dislike_count FROM posts WHERE id = ?", int64(post_id))
	err = rowCounts.Scan(&likeCount, &dislikeCount)
	recordError2(err)
	if err != nil {
		return 0, 0, fmt.Errorf("error reading updated counts: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("error committing transaction: %v", err)
	}

	return likeCount, dislikeCount, nil
}

func DeletePost(db *sql.DB, user_id model.AccountID, post_id model.PostID) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 500, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	var postOwnerID uuid.UUID
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

	if model.AccountID(postOwnerID) != user_id {
		return 403, fmt.Errorf("user is not authorized to delete this post")
	}

	_, err = database.ExecWithMetricsTx(tx, "delete_post", "DELETE FROM posts WHERE id = ?", int64(post_id))
	if err != nil {
		return 500, fmt.Errorf("error deleting post: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return 500, fmt.Errorf("error committing transaction: %v", err)
	}

	return 200, nil
}

