package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// Server wraps an HTTP server with graceful shutdown.
type Server struct {
	cfg    *ServerConfig
	engine *gin.Engine
	srv    *http.Server
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	Host string
	Port string
	Env  string
}

// New creates a new server.
func New(cfg *ServerConfig) *Server {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())

	return &Server{
		cfg:    cfg,
		engine: engine,
	}
}

// Router returns the Gin engine for route registration.
func (s *Server) Router() *gin.Engine {
	return s.engine
}

// Addr returns the server address.
func (s *Server) Addr() string {
	return s.cfg.Host + ":" + s.cfg.Port
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.srv = &http.Server{
		Addr:    s.Addr(),
		Handler: s.engine,
	}

	slog.Info("server starting", "addr", s.Addr())
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

// WaitForShutdown blocks until a shutdown signal is received, then gracefully shuts down.
func (s *Server) WaitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if s.srv != nil {
		if err := s.srv.Shutdown(ctx); err != nil {
			slog.Error("server forced to shutdown", "error", err)
		}
	}
	slog.Info("server exited")
}
