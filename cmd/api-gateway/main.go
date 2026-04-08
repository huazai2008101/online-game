package main

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

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

	// WebSocket proxy: /ws -> game service
	gameServiceURL := envOr("GAME_SERVICE_URL", "http://localhost:8002")
	r.GET("/ws", wsProxy(gameServiceURL))

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

	// Lobby SPA: serve built Vue app
	lobbyDist := envOr("LOBBY_DIST_PATH", "./web/lobby/dist")
	r.Static("/assets", lobbyDist+"/assets")
	r.StaticFile("/favicon.svg", lobbyDist+"/favicon.svg")
	r.StaticFile("/vite.svg", lobbyDist+"/vite.svg")
	r.NoRoute(func(c *gin.Context) {
		// SPA fallback: serve index.html for all unmatched routes
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") &&
			!strings.HasPrefix(c.Request.URL.Path, "/ws") &&
			!strings.HasPrefix(c.Request.URL.Path, "/game-assets") {
			c.File(lobbyDist + "/index.html")
			return
		}
		c.JSON(404, gin.H{"error": "not found"})
	})

	go srv.Start()
	srv.WaitForShutdown()
}

// wsProxy creates a Gin handler that proxies WebSocket connections to a backend.
func wsProxy(backendURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		target, _ := url.Parse(backendURL)

		// Build backend WS URL
		wsScheme := "ws"
		if target.Scheme == "https" {
			wsScheme = "wss"
		}
		backendWSURL := wsScheme + "://" + target.Host + c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			backendWSURL += "?" + c.Request.URL.RawQuery
		}

		// Dial backend
		backendConn, _, err := websocket.DefaultDialer.Dial(backendWSURL, nil)
		if err != nil {
			slog.Error("ws proxy: backend dial failed", "error", err)
			c.JSON(502, gin.H{"error": "failed to connect to game service"})
			return
		}
		defer backendConn.Close()

		// Upgrade client connection
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		}
		clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			slog.Error("ws proxy: client upgrade failed", "error", err)
			return
		}
		defer clientConn.Close()

		// Bidirectional proxy
		done := make(chan struct{}, 2)
		go proxyWSDir(backendConn, clientConn, done)
		go proxyWSDir(clientConn, backendConn, done)

		<-done
		slog.Debug("ws proxy: connection closed")
	}
}

// proxyWSDir copies messages from src to dst WebSocket connection.
func proxyWSDir(dst, src *websocket.Conn, done chan struct{}) {
	defer func() { done <- struct{}{} }()
	for {
		msgType, msg, err := src.ReadMessage()
		if err != nil {
			return
		}
		if err := dst.WriteMessage(msgType, msg); err != nil {
			return
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
