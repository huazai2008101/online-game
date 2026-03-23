package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Status represents health check status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Check represents a health check function
type Check func() error

// CheckResult represents the result of a health check
type CheckResult struct {
	Name     string    `json:"name"`
	Status   Status    `json:"status"`
	Message  string    `json:"message,omitempty"`
	Duration int64     `json:"duration_ms"`
	CheckedAt time.Time `json:"checked_at"`
}

// HealthResponse represents the overall health response
type HealthResponse struct {
	Status    Status                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks"`
	Version   string                 `json:"version,omitempty"`
}

// Checker manages health checks
type Checker struct {
	checks map[string]Check
	mu     sync.RWMutex
	version string
}

// NewChecker creates a new health checker
func NewChecker(version string) *Checker {
	return &Checker{
		checks:  make(map[string]Check),
		version: version,
	}
}

// Register registers a new health check
func (c *Checker) Register(name string, check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

// Unregister removes a health check
func (c *Checker) Unregister(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.checks, name)
}

// Check runs all health checks and returns the result
func (c *Checker) Check() *HealthResponse {
	c.mu.RLock()
	checks := make(map[string]Check, len(c.checks))
	for name, check := range c.checks {
		checks[name] = check
	}
	c.mu.RUnlock()

	response := &HealthResponse{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
		Checks:    make(map[string]CheckResult),
		Version:   c.version,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, check := range checks {
		wg.Add(1)
		go func(name string, check Check) {
			defer wg.Done()

			start := time.Now()
			err := check()
			duration := time.Since(start)

			result := CheckResult{
				Name:     name,
				Status:   StatusHealthy,
				Duration: duration.Milliseconds(),
				CheckedAt: time.Now(),
			}

			if err != nil {
				result.Status = StatusUnhealthy
				result.Message = err.Error()
			}

			mu.Lock()
			response.Checks[name] = result
			mu.Unlock()
		}(name, check)
	}

	wg.Wait()

	// Determine overall status
	for _, result := range response.Checks {
		if result.Status == StatusUnhealthy {
			response.Status = StatusUnhealthy
			return response
		}
		if result.Status == StatusDegraded {
			response.Status = StatusDegraded
		}
	}

	return response
}

// Handler returns an HTTP handler for health checks
func (c *Checker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := c.Check()

		w.Header().Set("Content-Type", "application/json")

		statusCode := http.StatusOK
		if response.Status == StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		} else if response.Status == StatusDegraded {
			statusCode = 207 // Multi-Status
		}

		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(response)
	}
}

// LiveHandler returns a simple liveness probe handler
func (c *Checker) LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
			"time":   time.Now().Format(time.RFC3339),
		})
	}
}

// ReadyHandler returns a readiness probe handler
func (c *Checker) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := c.Check()

		w.Header().Set("Content-Type", "application/json")

		if response.Status == StatusHealthy {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "ready",
				"checks": response.Checks,
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "not_ready",
				"checks": response.Checks,
			})
		}
	}
}

// Common health checks

// DBCheck creates a database health check
func DBCheck(ping func() error) Check {
	return func() error {
		return ping()
	}
}

// RedisCheck creates a Redis health check
func RedisCheck(ping func() error) Check {
	return func() error {
		return ping()
	}
}

// KafkaCheck creates a Kafka health check
func KafkaCheck(ping func() error) Check {
	return func() error {
		return ping()
	}
}

// HTTPCheck creates an HTTP endpoint health check
func HTTPCheck(url string, timeout time.Duration) Check {
	return func() error {
		client := &http.Client{
			Timeout: timeout,
		}
		resp, err := client.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("HTTP check failed with status %d", resp.StatusCode)
		}
		return nil
	}
}

// CustomCheck creates a custom health check from a function
func CustomCheck(fn func() error) Check {
	return fn
}
