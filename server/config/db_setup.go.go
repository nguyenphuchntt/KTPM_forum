package config

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"forum/server/database"
)

// migrationDir holds every migration, run in filename order. The names are
// timestamp-prefixed, so lexicographic order is chronological order: the schema
// file drops and recreates every table, and the FTS migration that follows
// alters the posts table it just created.
const migrationDir = "server/repository/postgresql/migration/"

// CreateTables executes every migration in the migration directory.
//
// PostgreSQL's extended protocol (used by the pgx driver) rejects multiple
// statements in a single Exec, so each file is split on ";" and each statement
// runs in order. The migrations have no semicolons inside string literals or
// function bodies we care about beyond the plpgsql block, which we keep whole.
func CreateTables(db *sql.DB) error {
	names, err := migrationFiles()
	if err != nil {
		return err
	}

	for _, name := range names {
		content, err := os.ReadFile(BasePath + migrationDir + name)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %v", name, err)
		}

		for _, statement := range splitSQLStatements(string(content)) {
			if _, err = db.Exec(statement); err != nil {
				return fmt.Errorf("migration %s: failed to run statement %.80q: %v", name, statement, err)
			}
		}
	}

	log.Println("Database schema created successfully")
	return nil
}

// migrationFiles returns the *.sql files in the migration directory, sorted so
// they run oldest first.
func migrationFiles() ([]string, error) {
	entries, err := os.ReadDir(BasePath + migrationDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration directory: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	return names, nil
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
