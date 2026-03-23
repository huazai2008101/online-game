package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Config holds the gateway configuration
type Config struct {
	Port            int
	Mode            string // debug, release, test
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	TracingEnabled  bool
	MetricsEnabled  bool
}

// DefaultConfig returns default gateway configuration
func DefaultConfig() *Config {
	return &Config{
		Port:            8080,
		Mode:            "release",
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		TracingEnabled:  true,
		MetricsEnabled:  true,
	}
}

// ServiceBackend represents a backend service
type ServiceBackend struct {
	Name        string
	Prefix      string
	TargetURL   string
	HealthURL   string
	KeepAlive   time.Duration
	MaxIdle     int
	proxy       *httputil.ReverseProxy
	initialized bool
	mu          sync.RWMutex
	healthy     bool
}

// Gateway is the API Gateway
type Gateway struct {
	config       *Config
	router       *gin.Engine
	backends     map[string]*ServiceBackend
	mu           sync.RWMutex
	server       *http.Server
	middlewares  []gin.HandlerFunc
	rateLimiter  *RateLimiter
	circuitBreaker *CircuitBreaker
}

// NewGateway creates a new API Gateway
func NewGateway(config *Config) *Gateway {
	if config == nil {
		config = DefaultConfig()
	}

	gin.SetMode(config.Mode)

	gw := &Gateway{
		config:      config,
		backends:    make(map[string]*ServiceBackend),
		middlewares: make([]gin.HandlerFunc, 0),
		rateLimiter: NewRateLimiter(1000, time.Hour), // 1000 requests per hour
		circuitBreaker: NewCircuitBreaker(CircuitBreakerConfig{
			MaxFailures: 5,
			ResetTimeout: 30 * time.Second,
		}),
	}

	gw.setupRouter()
	return gw
}

// setupRouter sets up the Gin router
func (gw *Gateway) setupRouter() {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(gw.loggerMiddleware())
	router.Use(gw.corsMiddleware())
	router.Use(gw.metricsMiddleware())

	// Health check endpoint
	router.GET("/health", gw.healthCheck)
	router.GET("/live", gw.liveProbe)
	router.GET("/ready", gw.readyProbe)

	// Metrics endpoint
	if gw.config.MetricsEnabled {
		router.GET("/metrics", gw.metricsHandler)
	}

	// API routes
	api := router.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			// Service routes will be registered here
			v1.Any("/*path", gw.proxyHandler)
		}
	}

	// WebSocket upgrade endpoint
	router.GET("/ws", gw.websocketHandler)

	gw.router = router
}

// RegisterBackend registers a service backend
func (gw *Gateway) RegisterBackend(backend *ServiceBackend) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	targetURL, err := url.Parse(backend.TargetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorHandler = gw.proxyErrorHandler

	// Customize transport
	proxy.Transport = NewTransport(backend.KeepAlive, backend.MaxIdle)

	backend.proxy = proxy
	backend.initialized = true
	backend.healthy = true

	gw.backends[backend.Name] = backend

	return nil
}

// UnregisterBackend removes a service backend
func (gw *Gateway) UnregisterBackend(name string) {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	delete(gw.backends, name)
}

// GetBackend returns a backend by name
func (gw *Gateway) GetBackend(name string) (*ServiceBackend, bool) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	backend, ok := gw.backends[name]
	return backend, ok
}

// Use adds a global middleware
func (gw *Gateway) Use(middleware gin.HandlerFunc) {
	gw.middlewares = append(gw.middlewares, middleware)
}

// Start starts the gateway
func (gw *Gateway) Start() error {
	gw.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", gw.config.Port),
		Handler:      gw.router,
		ReadTimeout:  gw.config.ReadTimeout,
		WriteTimeout: gw.config.WriteTimeout,
	}

	// Start health check for backends
	go gw.healthCheckLoop()

	return gw.server.ListenAndServe()
}

// Shutdown gracefully shuts down the gateway
func (gw *Gateway) Shutdown(ctx context.Context) error {
	if gw.server == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), gw.config.ShutdownTimeout)
	defer cancel()

	return gw.server.Shutdown(shutdownCtx)
}

// proxyHandler handles proxying requests to backend services
func (gw *Gateway) proxyHandler(c *gin.Context) {
	path := c.Request.URL.Path
	_ = c.Request.Method

	// Extract service name from path
	// /api/v1/users/... -> user-service
	// /api/v1/games/... -> game-service
	serviceName := gw.extractServiceName(path)

	backend, ok := gw.GetBackend(serviceName)
	if !ok {
		c.JSON(http.StatusBadGateway, gin.H{
			"code":    50201,
			"message": fmt.Sprintf("Service '%s' not found", serviceName),
		})
		return
	}

	// Check circuit breaker
	if !gw.circuitBreaker.Allow(serviceName) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    50301,
			"message": "Service circuit breaker open",
		})
		return
	}

	// Check rate limiting
	if !gw.rateLimiter.Allow(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    42901,
			"message": "Rate limit exceeded",
		})
		return
	}

	// Check backend health
	if !backend.IsHealthy() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    50302,
			"message": "Service unhealthy",
		})
		return
	}

	// Record start time
	start := time.Now()

	// Proxy request
	backend.proxy.ServeHTTP(c.Writer, c.Request)

	// Record metrics
	duration := time.Since(start)
	gw.circuitBreaker.RecordResult(serviceName, duration, nil)
}

// extractServiceName extracts service name from request path
func (gw *Gateway) extractServiceName(path string) string {
	// Remove /api/v1/ prefix
	prefix := "/api/v1/"
	if !strings.HasPrefix(path, prefix) {
		return "unknown"
	}

	remaining := strings.TrimPrefix(path, prefix)

	// Extract first segment
	parts := strings.Split(remaining, "/")
	if len(parts) == 0 {
		return "unknown"
	}

	// Map path prefixes to service names
	serviceMap := map[string]string{
		"users":        "user-service",
		"user":         "user-service",
		"games":        "game-service",
		"game":         "game-service",
		"rooms":        "game-service",
		"payments":     "payment-service",
		"payment":      "payment-service",
		"orders":       "payment-service",
		"scores":       "payment-service",
		"players":      "player-service",
		"player":       "player-service",
		"activities":   "activity-service",
		"activity":     "activity-service",
		"guilds":       "guild-service",
		"guild":        "guild-service",
		"items":        "item-service",
		"item":         "item-service",
		"notifications": "notification-service",
		"notification": "notification-service",
		"organizations": "organization-service",
		"organization": "organization-service",
		"orgs":         "organization-service",
		"permissions":  "permission-service",
		"permission":   "permission-service",
		"roles":        "permission-service",
		"files":        "file-service",
		"file":         "file-service",
		"upload":       "file-service",
		"id":           "id-service",
	}

	firstPart := parts[0]
	if service, ok := serviceMap[firstPart]; ok {
		return service
	}

	return "unknown"
}

// IsHealthy returns the health status of the backend
func (b *ServiceBackend) IsHealthy() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.healthy
}

// SetHealthy sets the health status of the backend
func (b *ServiceBackend) SetHealthy(healthy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.healthy = healthy
}

// HealthCheck performs health check on the backend
func (b *ServiceBackend) HealthCheck() error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	healthURL := b.HealthURL
	if healthURL == "" {
		healthURL = b.TargetURL + "/health"
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		b.SetHealthy(false)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		b.SetHealthy(false)
		return fmt.Errorf("service unhealthy: status %d", resp.StatusCode)
	}

	b.SetHealthy(true)
	return nil
}

// healthCheckLoop runs periodic health checks
func (gw *Gateway) healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		gw.mu.RLock()
		backends := make(map[string]*ServiceBackend)
		for name, backend := range gw.backends {
			backends[name] = backend
		}
		gw.mu.RUnlock()

		for _, backend := range backends {
			go func(b *ServiceBackend) {
				_ = b.HealthCheck()
			}(backend)
		}
	}
}

// proxyErrorHandler handles proxy errors
func (gw *Gateway) proxyErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusBadGateway)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"code":50200,"message":"Proxy error: %v"}`, err)))
}

// loggerMiddleware logs requests
func (gw *Gateway) loggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		// Log: [Method] Path - Status - Latency
		fmt.Printf("[%s] %s?%s - %d - %v\n",
			c.Request.Method, path, query, status, latency)
	}
}

// corsMiddleware handles CORS
func (gw *Gateway) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// metricsMiddleware records request metrics
func (gw *Gateway) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		// Record metrics here (e.g., to Prometheus)
		_ = time.Since(start)
	}
}

// healthCheck returns gateway health status
func (gw *Gateway) healthCheck(c *gin.Context) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	status := "healthy"
	checks := make(map[string]interface{})

	// Check all backends
	for name, backend := range gw.backends {
		healthy := backend.IsHealthy()
		checks[name] = map[string]interface{}{
			"status":    map[bool]string{true: "healthy", false: "unhealthy"}[healthy],
			"target":    backend.TargetURL,
		}
		if !healthy {
			status = "degraded"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    status,
		"timestamp": time.Now().Format(time.RFC3339),
		"checks":    checks,
	})
}

// liveProbe returns liveness status
func (gw *Gateway) liveProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// readyProbe returns readiness status
func (gw *Gateway) readyProbe(c *gin.Context) {
	gw.mu.RLock()
	ready := len(gw.backends) > 0
	gw.mu.RUnlock()

	if ready {
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
		})
	}
}

// metricsHandler returns Prometheus metrics
func (gw *Gateway) metricsHandler(c *gin.Context) {
	// Return Prometheus metrics here
	c.String(http.StatusOK, "# Metrics endpoint\n")
}

// websocketHandler handles WebSocket connections
func (gw *Gateway) websocketHandler(c *gin.Context) {
	// WebSocket upgrade handling
	c.JSON(http.StatusOK, gin.H{
		"message": "WebSocket endpoint - upgrade required",
	})
}
