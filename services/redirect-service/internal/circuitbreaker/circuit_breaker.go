package circuitbreaker

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed   State = iota // Normal: requests flow through
	StateOpen                  // Tripped: requests blocked (fail fast)
	StateHalfOpen              // Testing: limited requests allowed
)

// Config holds circuit breaker configuration
type Config struct {
	MaxFailures     int           // Failures before opening circuit
	ResetTimeout    time.Duration // How long to stay open before trying half-open
	HalfOpenMaxReqs int           // Max requests in half-open state
}

// CircuitBreaker implements the Circuit Breaker pattern
// Protects downstream services from cascade failures
//
// State machine:
//   CLOSED → (too many failures) → OPEN
//   OPEN   → (timeout elapsed)   → HALF-OPEN
//   HALF-OPEN → (success)        → CLOSED
//   HALF-OPEN → (failure)        → OPEN
type CircuitBreaker struct {
	name         string
	config       Config
	state        State
	failures     int
	halfOpenReqs int
	lastFailAt   time.Time
	mu           sync.Mutex
	logger       *zap.Logger
}

func New(name string, config Config, logger *zap.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:   name,
		config: config,
		state:  StateClosed,
		logger: logger,
	}
}

// Allow returns true if a request should be allowed through
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if reset timeout has elapsed
		if time.Since(cb.lastFailAt) >= cb.config.ResetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenReqs = 0
			cb.logger.Info("circuit breaker: OPEN → HALF-OPEN", zap.String("name", cb.name))
			return true
		}
		return false // Fail fast

	case StateHalfOpen:
		if cb.halfOpenReqs < cb.config.HalfOpenMaxReqs {
			cb.halfOpenReqs++
			return true
		}
		return false
	}
	return false
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.logger.Info("circuit breaker: HALF-OPEN → CLOSED", zap.String("name", cb.name))
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailAt = time.Now()

	if cb.state == StateHalfOpen || cb.failures >= cb.config.MaxFailures {
		cb.state = StateOpen
		cb.logger.Warn("circuit breaker: OPEN",
			zap.String("name", cb.name),
			zap.Int("failures", cb.failures),
		)
	}
}

// State returns current circuit state as string (for metrics)
func (cb *CircuitBreaker) StateString() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return "unknown"
}
