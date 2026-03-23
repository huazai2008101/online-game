// Package server provides the base server implementation for all services
package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"online-game/pkg/api"
	"online-game/pkg/config"
)

// Server represents the base HTTP server
type Server struct {
	config     *config.Config
	router     *gin.Engine
	httpServer *http.Server
}

// New creates a new server
func New(cfg *config.Config) *Server {
	// Set Gin mode
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(corsMiddleware())

	return &Server{
		config: cfg,
		router: router,
	}
}

// Router returns the Gin router
func (s *Server) Router() *gin.Engine {
	return s.router
}

// RegisterRoutes registers routes for the service
func (s *Server) RegisterRoutes(registerFunc func(*gin.RouterGroup)) {
	v1 := s.router.Group("/api/v1")
	registerFunc(v1)

	// Health check endpoint
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"service":   s.config.ServiceName,
			"timestamp": time.Now().Unix(),
		})
	})

	// Ready check endpoint
	s.router.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ready",
			"service": s.config.ServiceName,
		})
	})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:           s.config.GetAddr(),
		Handler:        s.router,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	fmt.Printf("[%s] Starting server on %s\n", s.config.ServiceName, s.config.GetAddr())

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	fmt.Printf("[%s] Shutting down server...\n", s.config.ServiceName)
	return s.httpServer.Shutdown(ctx)
}

// WaitForShutdown waits for interrupt signal and shuts down the server
func (s *Server) WaitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		fmt.Printf("[%s] Server forced to shutdown: %v\n", s.config.ServiceName, err)
	}
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// BindJSON binds JSON body to a struct with validation
func BindJSON(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindJSON(obj); err != nil {
		api.ValidationError(c, err)
		return err
	}
	return nil
}

// BindQuery binds query parameters to a struct with validation
func BindQuery(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindQuery(obj); err != nil {
		api.ValidationError(c, err)
		return err
	}
	return nil
}

// BindURI binds URI parameters to a struct with validation
func BindURI(c *gin.Context, obj interface{}) error {
	if err := c.ShouldBindUri(obj); err != nil {
		api.ValidationError(c, err)
		return err
	}
	return nil
}
