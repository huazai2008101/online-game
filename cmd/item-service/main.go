// Item Service - 道具服务
package main

import (
	"log"
	"online-game/internal/item"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	cfg := config.Load("item-service")
	cfg.Port = 8007
	database, _ := db.New(&cfg.Database)
	defer database.Close()
	database.AutoMigration(&item.Item{}, &item.UserItem{})
	srv := server.New(cfg)
	srv.RegisterRoutes(item.NewHandler().RegisterRoutes)
	go srv.Start()
	log.Printf("Item Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
