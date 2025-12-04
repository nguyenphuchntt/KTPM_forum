package config

import (
	"os"
	"strconv"
	"time"
)

type DBPoolConfig struct {
	MaxOpenConns    int          
	MaxIdleConns    int          
	ConnMaxLifetime time.Duration 
	ConnMaxIdleTime time.Duration 
}


func DefaultDBPoolConfig() *DBPoolConfig {
	return &DBPoolConfig{
		MaxOpenConns:    25,             
		MaxIdleConns:    10,              
		ConnMaxLifetime: 5 * time.Minute, 
		ConnMaxIdleTime: 2 * time.Minute, 
	}
}

func LoadDBPoolConfigFromEnv() *DBPoolConfig {
	config := DefaultDBPoolConfig()

	if maxOpenConns := os.Getenv("DB_MAX_OPEN_CONNS"); maxOpenConns != "" {
		if val, err := strconv.Atoi(maxOpenConns); err == nil && val > 0 {
			config.MaxOpenConns = val
		}
	}

	if maxIdleConns := os.Getenv("DB_MAX_IDLE_CONNS"); maxIdleConns != "" {
		if val, err := strconv.Atoi(maxIdleConns); err == nil && val > 0 {
			config.MaxIdleConns = val
		}
	}

	if connMaxLifetime := os.Getenv("DB_CONN_MAX_LIFETIME_MINUTES"); connMaxLifetime != "" {
		if val, err := strconv.Atoi(connMaxLifetime); err == nil && val > 0 {
			config.ConnMaxLifetime = time.Duration(val) * time.Minute
		}
	}

	if connMaxIdleTime := os.Getenv("DB_CONN_MAX_IDLE_TIME_MINUTES"); connMaxIdleTime != "" {
		if val, err := strconv.Atoi(connMaxIdleTime); err == nil && val > 0 {
			config.ConnMaxIdleTime = time.Duration(val) * time.Minute
		}
	}

	if config.MaxIdleConns > config.MaxOpenConns {
		config.MaxIdleConns = config.MaxOpenConns
	}

	return config
}
