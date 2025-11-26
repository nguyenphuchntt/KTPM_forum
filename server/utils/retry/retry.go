package retry

import (
	"context"
	"fmt"
	"log"
	"time"
)

func Try(ctx context.Context, cfg RetryConfig, callback func() error) error {
	var lastError error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %v", ContextCancelled, ctx.Err())
		default:
		}

		err := callback()
		lastError = err

		if err == nil {
			if attempt > 1 {
				log.Printf("Success on attempt %d/%d", attempt, cfg.MaxAttempts)
			}
			return nil
		}
		// was an error
		
		if IsNonRetryableError(err) {
			log.Printf("Non-retryable error occurred %v", err)
			return err
		}
		
		if attempt >= cfg.MaxAttempts {
			log.Printf("Max attempts reached. Last error:%v", err)
			return fmt.Errorf("%w: %v", MaxAttemptsReached, err)
		}

		delay := CalculateDelay(cfg, attempt)
		log.Printf("Attempt %d failed with error %v, retrying in %v", attempt, err, delay)

		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %v", ContextCancelled, ctx.Err())
		case <-time.After(delay):
		}
	}

	return lastError
}

func TryWithResult[T any] (ctx context.Context, cfg RetryConfig, callback func() (T, error)) (T, error) {
	var result T
	var lastError error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return result, fmt.Errorf("%w: %v", ContextCancelled, ctx.Err())
		default:
		}

		res, err := callback()
		lastError = err

		if err == nil {
			if attempt > 1 {
				log.Printf("Success on attempt %d/%d", attempt, cfg.MaxAttempts)
			}
			return res, nil
		}
		if IsNonRetryableError(err) {
			log.Printf("Non-retryable error occurred %v", err)
			return result, err			
		}
		if attempt >= cfg.MaxAttempts {
			log.Printf("Max attempts reached. Last error:%v", err)
			return result, fmt.Errorf("%w: %v", MaxAttemptsReached, err)
		}

		delay := CalculateDelay(cfg, attempt)
		log.Printf("Attempt %d failed with error %v, retrying in %v", attempt, err, delay)

		select {
		case <-ctx.Done():
			return result, fmt.Errorf("%w: %v", ContextCancelled, ctx.Err())
		case <-time.After(delay):
		}
	}
	return result, lastError
}