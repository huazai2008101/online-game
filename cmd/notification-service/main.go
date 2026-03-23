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
	cfg.Database.Database = "game_notification_db"

	database, err := db.New(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := database.AutoMigration(&notification.Notification{}, &notification.Message{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize layers (Repository -> Service -> Handler)
	notifRepo := notification.NewNotificationRepositoryImpl(database.DB)
	msgRepo := notification.NewMessageRepositoryImpl(database.DB)
	service := notification.NewService(notifRepo, msgRepo)

	srv := server.New(cfg)
	handler := notification.NewHandler(service)
	srv.RegisterRoutes(handler.RegisterRoutes)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("Notification Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
