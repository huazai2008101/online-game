// Activity Service - 活动服务
package main

import (
	"log"
	"online-game/internal/activity"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	cfg := config.Load("activity-service")
	cfg.Port = 8005
	database, _ := db.New(&cfg.Database)
	defer database.Close()
	database.AutoMigration(&activity.Activity{}, &activity.ActivityReward{})
	srv := server.New(cfg)
	srv.RegisterRoutes(activity.NewHandler().RegisterRoutes)
	go srv.Start()
	log.Printf("Activity Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
