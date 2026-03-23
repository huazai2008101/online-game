// Guild Service - 公会服务
package main

import (
	"log"
	"online-game/internal/guild"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	cfg := config.Load("guild-service")
	cfg.Port = 8006
	database, _ := db.New(&cfg.Database)
	defer database.Close()
	database.AutoMigration(&guild.Guild{}, &guild.GuildMember{})
	srv := server.New(cfg)
	srv.RegisterRoutes(guild.NewHandler().RegisterRoutes)
	go srv.Start()
	log.Printf("Guild Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
