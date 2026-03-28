package main

import (
	"log/slog"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"online-game/internal/server"
	"online-game/pkg/config"
)

func main() {
	cfg := config.Load("api-gateway")
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	slog.Info("starting api-gateway", "port", cfg.Port)

	srv := server.New(&server.ServerConfig{Host: cfg.Host, Port: cfg.Port, Env: cfg.Env})
	r := srv.Router()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "api-gateway"})
	})

	// Service backends (configurable via env)
	backends := map[string]string{
		"users": envOr("USER_SERVICE_URL", "http://localhost:8001"),
		"auth":  envOr("USER_SERVICE_URL", "http://localhost:8001"),
		"games": envOr("GAME_SERVICE_URL", "http://localhost:8002"),
		"rooms": envOr("GAME_SERVICE_URL", "http://localhost:8002"),
		"admin": envOr("ADMIN_SERVICE_URL", "http://localhost:8003"),
	}

	// Reverse proxy: /api/v1/{service}/*path -> service backend
	r.Any("/api/v1/*path", func(c *gin.Context) {
		path := strings.TrimPrefix(c.Param("path"), "/")
		segments := strings.SplitN(path, "/", 2)
		if len(segments) == 0 || segments[0] == "" {
			c.JSON(400, gin.H{"error": "invalid path"})
			return
		}

		serviceName := segments[0]
		backendURL, ok := backends[serviceName]
		if !ok {
			c.JSON(404, gin.H{"error": "service not found: " + serviceName})
			return
		}

		target, _ := url.Parse(backendURL)
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	// Game frontend static files
	gameAssets := envOr("GAME_ASSETS_PATH", "./data/games")
	r.Static("/game-assets", gameAssets)

	go srv.Start()
	srv.WaitForShutdown()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
