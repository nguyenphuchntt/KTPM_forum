package cache

import (
	"database/sql"
	"log"
	"sync"
	"time"
)

type CategoryInfo struct {
	ID         int
	Label      string
	PostsCount int
}

type CategoryCache struct {
	mu            sync.RWMutex
	categories    []CategoryInfo
	categoryMap   map[int]CategoryInfo 
	lastRefresh   time.Time
	refreshTTL    time.Duration
}

func NewCategoryCache(ttl time.Duration) *CategoryCache {
	return &CategoryCache{
		categoryMap:   make(map[int]CategoryInfo),
		refreshTTL:    ttl,
	}
}

// LoadCategories fetches categories from database and caches them
func (cc *CategoryCache) LoadCategories(db *sql.DB) error {
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
	
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var categories []CategoryInfo
	categoryMap := make(map[int]CategoryInfo)

	for rows.Next() {
		var cat CategoryInfo
		if err := rows.Scan(&cat.ID, &cat.Label, &cat.PostsCount); err != nil {
			return err
		}
		categories = append(categories, cat)
		categoryMap[cat.ID] = cat
	}

	cc.mu.Lock()
	cc.categories = categories
	cc.categoryMap = categoryMap
	cc.lastRefresh = time.Now()
	cc.mu.Unlock()

	log.Printf("[CategoryCache] Loaded %d categories", len(categories))
	return nil
}

// GetAll returns all cached categories
func (cc *CategoryCache) GetAll() []CategoryInfo {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	// Return copy to prevent mutation
	result := make([]CategoryInfo, len(cc.categories))
	copy(result, cc.categories)
	return result
}

// Exists checks if a category ID exists in cache
func (cc *CategoryCache) Exists(id int) bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	_, exists := cc.categoryMap[id]
	return exists
}

// ValidateIDs checks if all provided IDs exist
func (cc *CategoryCache) ValidateIDs(ids []int) bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	for _, id := range ids {
		if _, exists := cc.categoryMap[id]; !exists {
			return false
		}
	}
	return true
}

// ShouldRefresh checks if cache needs refresh
func (cc *CategoryCache) ShouldRefresh() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	return time.Since(cc.lastRefresh) > cc.refreshTTL
}

// StartAutoRefresh starts background refresh goroutine
func (cc *CategoryCache) StartAutoRefresh(db *sql.DB) {
	go func() {
		ticker := time.NewTicker(cc.refreshTTL)
		defer ticker.Stop()
		
		for range ticker.C {
			if err := cc.LoadCategories(db); err != nil {
				log.Printf("[CategoryCache] Auto-refresh failed: %v", err)
			}
		}
	}()
}
