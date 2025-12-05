package retry

import (
	"time"
	"errors"
)

var MaxAttemptsReached = errors.New("Maximum retry attempts reached")
var ContextCancelled = errors.New("Context cancelled during attempting retry")

type BackoffStrategy int
const (
	FixedDelay BackoffStrategy = iota
	LinearBackoff
	ExponentialBackoff
)

type RetryConfig struct {
	MaxAttempts      int
	InitialDelay   time.Duration
	MaxDelay time.Duration
	Strategy BackoffStrategy 
}

// default retry config
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay: 2 * time.Second,
		Strategy: ExponentialBackoff,
	}
}

// create connection to database retry config 
func InitDatabaseConnectionRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 20,
		InitialDelay: 1 * time.Second,
		MaxDelay: 30 * time.Second,
		Strategy: ExponentialBackoff,
	}
}

// database read operations 
func DatabaseQueryRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 5,
		InitialDelay: 200 * time.Millisecond,
		MaxDelay: 1 * time.Second,
		Strategy: ExponentialBackoff,
	}
}

// database write operations
func DatabaseWriteRetryConfig() RetryConfig {
	return RetryConfig {
		MaxAttempts: 3,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay: 1 * time.Second,
		Strategy: ExponentialBackoff,
	}
}

// func DatabaseTransactionConfig() RetryConfig {
// 	return RetryConfig{
// 		MaxAttempts:         3,
// 		InitialDelay:        100 * time.Millisecond,
// 		MaxDelay:            1 * time.Second,
// 		Strategy:            ExponentialBackoff,
// 	}
// }

func DatabaseSetupConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:         7,
		InitialDelay:        2 * time.Second,
		MaxDelay:            14 * time.Second,
		Strategy:            LinearBackoff,
	}
}
