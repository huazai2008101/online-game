// Player Service - 玩家服务
package main

import (
	"log"

	"online-game/internal/player"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	cfg := config.Load("player-service")
	cfg.Port = 8004
	cfg.Database.Database = "game_core_db"

	database, err := db.New(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := database.AutoMigration(&player.Player{}, &player.PlayerStats{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize layers (Repository -> Service -> Handler)
	playerRepo := player.NewPlayerRepositoryImpl(database.DB)
	statsRepo := player.NewPlayerStatsRepositoryImpl(database.DB)
	service := player.NewService(playerRepo, statsRepo)

	srv := server.New(cfg)
	handler := player.NewHandler(service)
	srv.RegisterRoutes(handler.RegisterRoutes)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("Player Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
