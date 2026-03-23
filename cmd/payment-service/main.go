// Payment Service - 支付服务
package main

import (
	"log"

	"online-game/internal/payment"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	cfg := config.Load("payment-service")
	cfg.Port = 8003
	cfg.Database.Database = "game_payment_db"

	database, err := db.New(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	database.AutoMigration(
		&payment.Order{},
		&payment.Score{},
		&payment.ScoreLog{},
	)

	// Initialize layers (Repository -> Service -> Handler)
	orderRepo := payment.NewOrderRepositoryImpl(database.DB)
	scoreRepo := payment.NewScoreRepositoryImpl(database.DB)
	service := payment.NewService(orderRepo, scoreRepo)

	srv := server.New(cfg)
	handler := payment.NewHandler(service)
	srv.RegisterRoutes(handler.RegisterRoutes)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("Payment Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
