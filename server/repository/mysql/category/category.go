package category

import (
	"database/sql"
	"fmt"
	"strings"

	"forum/server/cache"
	"forum/server/database"
	"forum/server/model"
)

func FetchCategories(db *sql.DB) ([]model.Category, error) {
	// Use cache if available
	if cache.GlobalCategoryCache != nil {
		cachedCategories := cache.GlobalCategoryCache.GetAll()
		if len(cachedCategories) > 0 {
			categories := make([]model.Category, len(cachedCategories))
			for i, cat := range cachedCategories {
				categories[i] = model.Category{
					ID:    model.CategoryID(cat.ID),
					Label: cat.Label,
				}
			}
			return categories, nil
		}
	}

	var categories []model.Category
	query := `
		SELECT
			c.id,
			c.label
		FROM categories c
		ORDER BY c.label ASC;
	`
	rows, err := database.QueryWithMetrics(db, "select_categories", query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			category model.Category
			id       int64
		)
		if err := rows.Scan(&id, &category.Label); err != nil {
			return nil, err
		}
		category.ID = model.CategoryID(id)
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
