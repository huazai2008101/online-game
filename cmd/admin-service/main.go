package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"online-game/internal/admin"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
	grpcserver "online-game/pkg/grpc"
	pb "online-game/proto/user"
)

func main() {
	cfg := config.Load("admin-service")
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	slog.Info("starting admin-service", "port", cfg.Port)

	database, err := db.New(&cfg.Database)
	if err != nil {
		slog.Error("database error", "error", err)
		os.Exit(1)
	}
	defer db.Close(database)

	// gRPC client to user-service for token validation
	var userClient *grpcserver.UserServiceClient
	if cfg.GRPCPort != "" {
		userServiceAddr := envOr("USER_GRPC_ADDR", "localhost:9001")
		uc, err := grpcserver.NewUserServiceClient(userServiceAddr)
		if err != nil {
			slog.Warn("user-service gRPC not available, admin auth disabled", "error", err)
		} else {
			userClient = uc
			defer uc.Close()
		}
	}

	storagePath := filepath.Join(".", "data", "games")
	os.MkdirAll(storagePath, 0755)

	adminSvc := admin.NewService(database, storagePath)
	adminSvc.Migrate()

	adminHandler := admin.NewHandler(adminSvc)

	srv := server.New(&server.ServerConfig{Host: cfg.Host, Port: cfg.Port, Env: cfg.Env})

	// Admin auth middleware: validate JWT via user-service gRPC
	if userClient != nil {
		adminMw := func(c *gin.Context) {
			token := c.GetHeader("Authorization")
			if len(token) > 7 && token[:7] == "Bearer " {
				token = token[7:]
			}
			if token == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
			resp, err := userClient.Client.ValidateToken(c.Request.Context(), &pb.ValidateTokenRequest{Token: token})
			if err != nil || !resp.Valid {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
				return
			}
			if resp.Role != "admin" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin only"})
				return
			}
			c.Set("user_id", resp.UserId)
			c.Set("username", resp.Username)
			c.Set("role", resp.Role)
			c.Next()
		}
		adminHandler.RegisterRoutes(srv.Router().Group("/api/v1", adminMw))
	} else {
		adminHandler.RegisterRoutes(srv.Router().Group("/api/v1"))
	}

	srv.Router().Static("/game-assets", storagePath)
	srv.Router().GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "admin-service"})
	})

	go srv.Start()
	srv.WaitForShutdown()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
