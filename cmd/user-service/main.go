// User Service - 用户服务
package main

import (
	"log"

	"online-game/internal/server"
	"online-game/internal/user"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	// Load configuration
	cfg := config.Load("user-service")
	cfg.Port = 8001
	cfg.Database.Database = "game_platform_db"

	// Initialize database
	database, err := db.New(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Auto migrate tables
	if err := database.AutoMigration(
		&user.User{},
		&user.UserProfile{},
		&user.Friend{},
		&user.UserSession{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize layers (Repository -> Service -> Handler)
	repo := user.NewRepository(database.DB)
	service := user.NewService(repo)
	handler := user.NewHandler(service)

	// Create server
	srv := server.New(cfg)

	// Register routes
	srv.RegisterRoutes(handler.RegisterRoutes)

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("User Service started on %s", cfg.GetAddr())

	// Wait for shutdown signal
	srv.WaitForShutdown()
}
