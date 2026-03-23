// Notification Service - 通知服务
package main

import (
	"log"
	"online-game/internal/notification"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	cfg := config.Load("notification-service")
	cfg.Port = 8008
	database, _ := db.New(&cfg.Database)
	defer database.Close()
	database.AutoMigration(&notification.Notification{}, &notification.Message{})
	srv := server.New(cfg)
	srv.RegisterRoutes(notification.NewHandler().RegisterRoutes)
	go srv.Start()
	log.Printf("Notification Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
