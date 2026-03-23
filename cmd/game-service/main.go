// Game Service - 核心游戏服务
package main

import (
	"log"

	"online-game/internal/game"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	// Load configuration
	cfg := config.Load("game-service")
	cfg.Port = 8002
	cfg.Database.Database = "game_core_db"

	// Initialize database
	database, err := db.New(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Auto migrate tables
	if err := database.AutoMigration(
		&game.Game{},
		&game.GameVersion{},
		&game.GameRoom{},
		&game.GameSession{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize repository and manager
	repo := game.NewRepository(database.DB)
	mgr := game.NewGameInstanceManager(database.DB)
	defer mgr.Shutdown()

	// Create server
	srv := server.New(cfg)

	// Register routes
	handler := game.NewHandler(repo, mgr)
	srv.RegisterRoutes(handler.RegisterRoutes)

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("Game Service started on %s", cfg.GetAddr())

	// Wait for shutdown signal
	srv.WaitForShutdown()
}
