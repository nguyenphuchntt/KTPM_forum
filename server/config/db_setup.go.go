package config

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"forum/server/database"
)

// CreateTables executes all queries from the new migration schema.
//
// PostgreSQL's extended protocol (used by the pgx driver) rejects multiple
// statements in a single Exec, so the file is split on ";" and each statement
// runs in order. The schema has no semicolons inside string literals or
// function bodies we care about beyond the plpgsql block, which we keep whole.
func CreateTables(db *sql.DB) error {
	content, err := os.ReadFile(BasePath + "server/repository/mysql/migration/20260916094300_schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration schema file: %v", err)
	}

	for _, statement := range splitSQLStatements(string(content)) {
		if _, err = db.Exec(statement); err != nil {
			return fmt.Errorf("failed to run schema statement %.80q: %v", statement, err)
		}
	}

	log.Println("Database schema created successfully")
	return nil
}

func splitSQLStatements(script string) []string {
	var statements []string
	var current strings.Builder
	inDollarQuote := false

	lines := strings.Split(script, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inDollarQuote && strings.HasPrefix(trimmed, "--") {
			continue
		}

		lineContent := line
		if !inDollarQuote {
			if idx := strings.Index(line, "--"); idx != -1 {
				lineContent = line[:idx]
			}
		}

		for i := 0; i < len(lineContent); i++ {
			if i+1 < len(lineContent) && lineContent[i] == '$' && lineContent[i+1] == '$' {
				inDollarQuote = !inDollarQuote
				current.WriteString("$$")
				i++
				continue
			}

			if lineContent[i] == ';' && !inDollarQuote {
				if stmt := strings.TrimSpace(current.String()); stmt != "" {
					statements = append(statements, stmt)
				}
				current.Reset()
				continue
			}

			current.WriteByte(lineContent[i])
		}
		current.WriteByte('\n')
	}

	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// CreateDemoData generates and inserts demo data using repository functions
// to ensure bcrypt password hashing and schema consistency.
func CreateDemoData(db *sql.DB) error {
	// create database schema before creating demo data
	if err := CreateTables(db); err != nil {
		return err
	}

	return seedDemoData(db)
}

// seedDemoData inserts minimal, self-consistent demo data against the new
// schema. Accounts are inserted directly with bcrypt hashes (userRepo.StoreUser
// cannot be imported because it pulls cache, which imports config). Categories
// use ON CONFLICT DO NOTHING so rerunning --seed does not error on UNIQUE labels.
// Idempotent: accounts are unique by username, and posts reference only the
// users created here.
func seedDemoData(db *sql.DB) error {
	// Categories (ON CONFLICT DO NOTHING so rerunning --seed does not error on
	// the UNIQUE label constraint).
	categoryLabels := []string{"General", "Go", "SQL", "DevOps"}
	for _, label := range categoryLabels {
		_, err := database.ExecWithMetrics(db, "seed_category",
			`INSERT INTO categories (label) VALUES (?) ON CONFLICT (label) DO NOTHING`, label)
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
			"Welcome to the forum", "How Go templates compose with PostgreSQL",
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
