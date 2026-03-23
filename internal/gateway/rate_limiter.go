package gateway

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	tokens   map[string]*bucket
	rate     int           // tokens per interval
	interval time.Duration // token refill interval
}

type bucket struct {
	tokens int
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		tokens:   make(map[string]*bucket),
		rate:     rate,
		interval: interval,
	}

	// Cleanup old buckets periodically
	go rl.cleanup()

	return rl
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	b, exists := rl.tokens[key]
	if !exists {
		rl.tokens[key] = &bucket{
			tokens:     rl.rate - 1,
			lastRefill: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(b.lastRefill)
	if elapsed >= rl.interval {
		// Full refill
		b.tokens = rl.rate
		b.lastRefill = now
	} else {
		// Partial refill
		refillTokens := int(elapsed.Milliseconds()) / int(rl.interval.Milliseconds()) * rl.rate
		b.tokens += refillTokens
		if b.tokens > rl.rate {
			b.tokens = rl.rate
		}
	}

	// Check if we have tokens
	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// cleanup removes old buckets
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, b := range rl.tokens {
			// Remove buckets not used in the last hour
			if now.Sub(b.lastRefill) > time.Hour {
				delete(rl.tokens, key)
			}
		}
		rl.mu.Unlock()
	}
}
