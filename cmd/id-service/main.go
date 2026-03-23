// ID Service - ID生成服务
package main

import (
	"log"
	"online-game/internal/id"
	"online-game/internal/server"
	"online-game/pkg/config"
)

func main() {
	cfg := config.Load("id-service")
	cfg.Port = 8011

	srv := server.New(cfg)
	srv.RegisterRoutes(id.NewHandler().RegisterRoutes)

	go srv.Start()
	log.Printf("ID Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
