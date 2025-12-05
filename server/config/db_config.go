package config

import (
	"context"
	"database/sql"
	"fmt"
	"forum/server/utils/retry"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/joho/godotenv"
)

func Connect() (*sql.DB, error) {
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	database := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true", user, password, host, port, database)
	log.Printf("Trying to connect to database")

	ctx, cancelF := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelF()

	retryConfig := retry.InitDatabaseConnectionRetryConfig()
	db, err := retry.TryWithResult(ctx, retryConfig, func() (*sql.DB, error) {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("failed open database connection with error: %v", err)
		}
		err = db.Ping()
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed ping database connection with error: %v", err)
		}

		return db, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed create database connection after maximum attempts with error: %v", err)
	}

	poolConfig := LoadDBPoolConfigFromEnv()
	db.SetMaxOpenConns(poolConfig.MaxOpenConns)
	db.SetMaxIdleConns(poolConfig.MaxIdleConns)
	db.SetConnMaxLifetime(poolConfig.ConnMaxLifetime)
	db.SetConnMaxIdleTime(poolConfig.ConnMaxIdleTime)

	log.Printf("Successfully create database connection")
	log.Printf("Connection pool configured: MaxOpen=%d, MaxIdle=%d, MaxLifetime=%v, MaxIdleTime=%v",
		poolConfig.MaxOpenConns, poolConfig.MaxIdleConns,
		poolConfig.ConnMaxLifetime, poolConfig.ConnMaxIdleTime)
	return db, nil
}
