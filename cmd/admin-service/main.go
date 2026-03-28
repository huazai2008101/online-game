package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"online-game/internal/admin"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
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

	storagePath := filepath.Join(".", "data", "games")
	os.MkdirAll(storagePath, 0755)

	adminSvc := admin.NewService(database, storagePath)
	adminSvc.Migrate()

	adminHandler := admin.NewHandler(adminSvc)

	srv := server.New(&server.ServerConfig{Host: cfg.Host, Port: cfg.Port, Env: cfg.Env})
	adminHandler.RegisterRoutes(srv.Router().Group("/api/v1"))
	srv.Router().Static("/game-assets", storagePath)
	srv.Router().GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "admin-service"})
	})

	go srv.Start()
	srv.WaitForShutdown()
}
