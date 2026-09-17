package config

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"forum/server/database"
	"forum/server/utils/retry"
)

// CreateTables executes all queries from the new migration schema
func CreateTables(db *sql.DB) error {
	ctx, cancelF := context.WithTimeout(context.Background(), 1 * time.Minute)
	defer cancelF()

	retryConfig := retry.DatabaseSetupConfig()
	return retry.Try(ctx, retryConfig, func() error {
		content, err := os.ReadFile(BasePath + "server/repository/mysql/migration/20260916094300_schema.sql")
		if err != nil {
			return fmt.Errorf("failed to read migration schema file: %v", err)
		}
		queries := strings.TrimSpace(string(content))
		_, err = db.Exec(queries)

		if err != nil {
			log.Printf("failed to create tables %q: %v\n", queries, err)
			return err
		}
		log.Println("Database schema created successfully")
		return nil
	})
}

// CreateDemoData generates and inserts demo data using repository functions
// to ensure bcrypt password hashing and schema consistency.
func CreateDemoData(db *sql.DB) error {
	// create database schema before creating demo data
	if err := CreateTables(db); err != nil {
		return err
	}

	ctx, cancelF := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelF()
	retryConfig := retry.DatabaseSetupConfig()

	return retry.Try(ctx, retryConfig, func() error {
		return seedDemoData(db)
	})
}

// seedDemoData inserts minimal, self-consistent demo data against the new
// schema. Accounts are inserted directly with bcrypt hashes (userRepo.StoreUser
// cannot be imported because it pulls cache, which imports config). Categories
// use INSERT IGNORE so rerunning --seed does not error on UNIQUE labels.
// Idempotent: accounts are unique by username, and posts reference only the
// users created here.
func seedDemoData(db *sql.DB) error {
	// Categories (INSERT IGNORE so rerunning --seed does not error on the
	// UNIQUE label constraint).
	categoryLabels := []string{"General", "Go", "SQL", "DevOps"}
	for _, label := range categoryLabels {
		_, err := database.ExecWithMetrics(db, "seed_category",
			`INSERT IGNORE INTO categories (label) VALUES (?)`, label)
		if err != nil {
			return fmt.Errorf("seed category %q: %w", label, err)
		}
	}

	// Accounts + their created posts.
	users := []struct {
		email, username, password string
		posts                     []string
	}{
		{"alice@example.com", "alice", "password1", []string{
			"Welcome to the forum", "How Go templates compose with MySQL",
		}},
		{"bob@example.com", "bob", "password2", []string{
			"Indexing notes for post lists",
		}},
	}

	for _, u := range users {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("seed user %q: hash password: %w", u.username, err)
		}
		accountID := uuid.New()
		if _, err := database.ExecWithMetrics(db, "seed_user",
			`INSERT INTO accounts (id, email, username, password) VALUES (?,?,?,?)`,
			accountID, u.email, u.username, hashedPassword); err != nil {
			return fmt.Errorf("seed user %q: %w", u.username, err)
		}
		for _, title := range u.posts {
			if _, err := database.ExecWithMetrics(db, "seed_post",
				`INSERT INTO posts (user_id, title, content) VALUES (?, ?, ?)`,
				accountID, title, "Demo content for "+title); err != nil {
				return fmt.Errorf("seed post %q: %w", title, err)
			}
		}
	}

	log.Println("Demo data created successfully")
	return nil
}

// Drop all tables in the database.
func Drop() error {
	err := os.Remove(BasePath + "server/database/database.db")
	if err != nil {
		log.Printf("failed to drop tables: %v\n", err)
		return err
	}

	log.Println("Database schema dropped successfully")
	return nil
}
