package gateway

import (
	"sync"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	MaxFailures  int           // Max failures before opening
	ResetTimeout time.Duration // Time to wait before trying again
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu             sync.RWMutex
	states         map[string]*circuitState
	config         CircuitBreakerConfig
}

type circuitState struct {
	state         CircuitBreakerState
	failures      int
	lastFailure   time.Time
	lastSuccess   time.Time
	openedAt      time.Time
	mu            sync.Mutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		states: make(map[string]*circuitState),
		config: config,
	}
}

// Allow checks if a request should be allowed through
func (cb *CircuitBreaker) Allow(service string) bool {
	state := cb.getOrCreateState(service)

	state.mu.Lock()
	defer state.mu.Unlock()

	// Check if we should try to reset from open to half-open
	if state.state == StateOpen {
		if time.Since(state.openedAt) >= cb.config.ResetTimeout {
			state.state = StateHalfOpen
			return true
		}
		return false
	}

	return true
}

// RecordResult records the result of a request
func (cb *CircuitBreaker) RecordResult(service string, duration time.Duration, err error) {
	state := cb.getOrCreateState(service)

	state.mu.Lock()
	defer state.mu.Unlock()

	if err != nil {
		state.failures++
		state.lastFailure = time.Now()

		// Open the circuit if too many failures
		if state.failures >= cb.config.MaxFailures {
			state.state = StateOpen
			state.openedAt = time.Now()
		}
	} else {
		// Reset on success
		if state.state == StateHalfOpen {
			state.state = StateClosed
		}
		state.failures = 0
		state.lastSuccess = time.Now()
	}
}

// GetState returns the current state of a service
func (cb *CircuitBreaker) GetState(service string) CircuitBreakerState {
	state := cb.getOrCreateState(service)
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.state
}

// getOrCreateState gets or creates a circuit state for a service
func (cb *CircuitBreaker) getOrCreateState(service string) *circuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if state, ok := cb.states[service]; ok {
		return state
	}

	state := &circuitState{
		state: StateClosed,
	}
	cb.states[service] = state
	return state
}

// Reset resets the circuit breaker for a service
func (cb *CircuitBreaker) Reset(service string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if state, ok := cb.states[service]; ok {
		state.mu.Lock()
		state.state = StateClosed
		state.failures = 0
		state.mu.Unlock()
	}
}
