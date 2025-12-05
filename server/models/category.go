package models

import (
	"database/sql"
	"fmt"
	"strings"

	"forum/server/cache"
	"forum/server/database"
)

type Category struct {
	ID         int
	Label      string
	PostsCount int
}

func FetchCategories(db *sql.DB) ([]Category, error) {
	// Use cache if available
	if cache.GlobalCategoryCache != nil {
		cachedCategories := cache.GlobalCategoryCache.GetAll()
		if len(cachedCategories) > 0 {
			// Convert cache.CategoryInfo to models.Category
			categories := make([]Category, len(cachedCategories))
			for i, cat := range cachedCategories {
				categories[i] = Category{
					ID:         cat.ID,
					Label:      cat.Label,
					PostsCount: cat.PostsCount,
				}
			}
			return categories, nil
		}
	}

	// Fallback to database query if cache not available
	var categories []Category
	query := `
		SELECT
			c.id,
			c.label,
			COUNT(pc.post_id) as posts_count
		FROM categories c
		LEFT JOIN post_category pc ON pc.category_id = c.id
		GROUP BY c.id, c.label
		ORDER BY posts_count DESC;
	`
	rows, err := database.QueryWithMetrics(db, "select_categories", query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var category Category
		rows.Scan(&category.ID, &category.Label, &category.PostsCount)
		categories = append(categories, category)
	}
	return categories, nil
}

func CheckCategories(db *sql.DB, ids []int) error {
	if cache.GlobalCategoryCache != nil {
		if cache.GlobalCategoryCache.ValidateIDs(ids) {
			return nil
		}
		return fmt.Errorf("categories does not exists in db")
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(`
        SELECT id
        FROM categories
        WHERE id IN (%s);
    `, placeholders)

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := database.QueryWithMetrics(db, "check_categories", query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		count++
	}
	if count != len(ids) {
		return fmt.Errorf("categories does not exists in db")
	}

	return nil
}
