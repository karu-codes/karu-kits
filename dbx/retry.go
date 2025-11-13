package dbx

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryStrategy defines the retry behavior
type RetryStrategy string

const (
	// RetryStrategyFixed uses fixed delay between retries
	RetryStrategyFixed RetryStrategy = "fixed"
	// RetryStrategyExponential uses exponential backoff
	RetryStrategyExponential RetryStrategy = "exponential"
	// RetryStrategyExponentialJitter uses exponential backoff with jitter
	RetryStrategyExponentialJitter RetryStrategy = "exponential_jitter"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts    int
	InitialDelay   time.Duration
	MaxDelay       time.Duration
	Strategy       RetryStrategy
	Multiplier     float64 // for exponential backoff
	OnRetry        func(attempt int, err error, delay time.Duration)
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Strategy:     RetryStrategyExponentialJitter,
		Multiplier:   2.0,
	}
}

// retryableOperation executes an operation with retry logic
func retryableOperation(ctx context.Context, cfg RetryConfig, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		// Execute operation
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return err
		}

		// Don't sleep after last attempt
		if attempt == cfg.MaxAttempts-1 {
			break
		}

		// Check if context is done
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Calculate delay
		delay := calculateDelay(cfg, attempt)

		// Call retry callback if provided
		if cfg.OnRetry != nil {
			cfg.OnRetry(attempt+1, err, delay)
		}

		// Sleep with context awareness
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(cfg RetryConfig, attempt int) time.Duration {
	var delay time.Duration

	switch cfg.Strategy {
	case RetryStrategyFixed:
		delay = cfg.InitialDelay

	case RetryStrategyExponential:
		delay = time.Duration(float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt)))

	case RetryStrategyExponentialJitter:
		exponentialDelay := time.Duration(float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt)))
		// Add jitter: random value between 0 and exponentialDelay
		jitter := time.Duration(rand.Int63n(int64(exponentialDelay)))
		delay = exponentialDelay + jitter

	default:
		delay = cfg.InitialDelay
	}

	// Cap at max delay
	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}

	return delay
}

// WithRetryConfig creates an option to set retry configuration
func WithRetryConfig(cfg RetryConfig) Option {
	return func(c *Config) {
		c.MaxRetries = cfg.MaxAttempts
		c.RetryDelay = cfg.InitialDelay
	}
}

// retryForConnection retries connection-specific operations
func retryForConnection(ctx context.Context, cfg Config, operation func() error) error {
	retryCfg := RetryConfig{
		MaxAttempts:  cfg.MaxRetries,
		InitialDelay: cfg.RetryDelay,
		MaxDelay:     30 * time.Second,
		Strategy:     RetryStrategyExponentialJitter,
		Multiplier:   2.0,
		OnRetry: func(attempt int, err error, delay time.Duration) {
			if cfg.Logger != nil {
				cfg.Logger.Printf("[dbx] connection retry attempt %d after error: %v (waiting %v)", attempt, err, delay)
			}
		},
	}

	return retryableOperation(ctx, retryCfg, operation)
}

// retryForTransaction retries transaction-specific operations (serialization, deadlock)
func retryForTransaction(ctx context.Context, cfg Config, operation func() error) error {
	retryCfg := RetryConfig{
		MaxAttempts:  cfg.MaxRetries,
		InitialDelay: cfg.RetryDelay,
		MaxDelay:     2 * time.Second,
		Strategy:     RetryStrategyExponentialJitter,
		Multiplier:   1.5,
		OnRetry: func(attempt int, err error, delay time.Duration) {
			if cfg.Logger != nil {
				cfg.Logger.Printf("[dbx] transaction retry attempt %d after error: %v (waiting %v)", attempt, err, delay)
			}
		},
	}

	return retryableOperation(ctx, retryCfg, operation)
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	failures     int
	lastFailTime time.Time
	state        CircuitBreakerState
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        CircuitBreakerClosed,
	}
}

// Execute runs an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(operation func() error) error {
	// Check if we should try to close the circuit
	if cb.state == CircuitBreakerOpen {
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = CircuitBreakerHalfOpen
			cb.failures = 0
		} else {
			return ErrCircuitBreakerOpen
		}
	}

	// Execute operation
	err := operation()

	// Handle result
	if err != nil {
		cb.failures++
		cb.lastFailTime = time.Now()

		if cb.failures >= cb.maxFailures {
			cb.state = CircuitBreakerOpen
		}
		return err
	}

	// Success - reset if we were in half-open state
	if cb.state == CircuitBreakerHalfOpen {
		cb.state = CircuitBreakerClosed
		cb.failures = 0
	}

	return nil
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitBreakerState {
	return cb.state
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.state = CircuitBreakerClosed
	cb.failures = 0
}

// Common errors
var (
	ErrCircuitBreakerOpen = newDBError("circuit breaker is open")
	ErrMaxRetriesExceeded = newDBError("max retries exceeded")
)

func newDBError(msg string) error {
	return &dbError{msg: msg}
}

type dbError struct {
	msg string
}

func (e *dbError) Error() string {
	return e.msg
}
