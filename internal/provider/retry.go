package provider

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// ---------------------------------------------------------------------------
// Retry helper
// ---------------------------------------------------------------------------

// retryable returns true if the error is worth retrying (rate limits, server
// errors, timeouts). Authentication and invalid-request errors are not
// retryable.
func retryable(err error) bool {
	var pe *ProviderError
	if !errors.As(err, &pe) {
		// Non-provider errors (network issues) are retried.
		return true
	}
	switch pe.Code {
	case ErrCodeRateLimit, ErrCodeProviderUnavailable, ErrCodeTimeout:
		return true
	default:
		return false
	}
}

// WithRetry wraps a function call with exponential backoff + jitter. If cfg
// has MaxRetries == 0 the function is called exactly once.
//
// Usage:
//
//	resp, err := provider.WithRetry(ctx, cfg, func() (*CompletionResponse, error) {
//	    return p.doRequest(ctx, req)
//	})
func WithRetry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	attempts := cfg.MaxRetries + 1 // first call + retries
	interval := cfg.InitialInterval
	if interval == 0 {
		interval = time.Second
	}

	for i := 0; i < attempts; i++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Do not retry non-retryable errors.
		if !retryable(err) {
			return zero, err
		}

		// Do not sleep after the last attempt.
		if i == attempts-1 {
			break
		}

		// Exponential backoff with jitter (full jitter strategy).
		jitter := time.Duration(rand.Int63n(int64(interval)))
		sleep := interval/2 + jitter

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(sleep):
		}

		// Grow interval for next round.
		interval = time.Duration(
			math.Min(
				float64(cfg.MaxInterval),
				float64(interval)*cfg.Multiplier,
			),
		)
	}

	return zero, lastErr
}
