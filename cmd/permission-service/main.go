// Permission Service - 权限服务
package main

import (
	"log"
	"online-game/internal/permission"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	cfg := config.Load("permission-service")
	cfg.Port = 8010
	database, _ := db.New(&cfg.Database)
	defer database.Close()
	database.AutoMigration(&permission.Role{}, &permission.Permission{}, &permission.RolePermission{}, &permission.UserRole{})
	srv := server.New(cfg)
	srv.RegisterRoutes(permission.NewHandler().RegisterRoutes)
	go srv.Start()
	log.Printf("Permission Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
