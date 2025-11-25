package retry

import (
	"math"
	"time"
)

func CalculateDelay(cfg RetryConfig, attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	var delay time.Duration

	switch cfg.Strategy {
	case FixedDelay:
		delay = cfg.InitialDelay
	case LinearBackoff:
		delay = time.Duration(attempt) * cfg.InitialDelay
	case ExponentialBackoff:
		multiplier := math.Pow(2, float64(attempt-1))
		delay = time.Duration(float64(cfg.InitialDelay) * multiplier)
	default:
		delay = cfg.InitialDelay
	}

	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}

	return delay
}