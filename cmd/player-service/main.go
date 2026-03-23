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

	database, _ := db.New(&cfg.Database)
	defer database.Close()
	database.AutoMigration(&player.Player{}, &player.PlayerStats{})

	repo := player.NewRepository(database.DB)
	srv := server.New(cfg)
	handler := player.NewHandler(repo)
	srv.RegisterRoutes(handler.RegisterRoutes)

	go srv.Start()
	log.Printf("Player Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
