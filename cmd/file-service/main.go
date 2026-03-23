// File Service - 文件服务
package main

import (
	"log"
	"online-game/internal/file"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	cfg := config.Load("file-service")
	cfg.Port = 8012
	cfg.Database.Database = "game_file_db"

	database, _ := db.New(&cfg.Database)
	defer database.Close()
	database.AutoMigration(&file.File{})

	srv := server.New(cfg)
	srv.RegisterRoutes(file.NewHandler().RegisterRoutes)

	go srv.Start()
	log.Printf("File Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
