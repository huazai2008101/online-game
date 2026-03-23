// Organization Service - 组织服务
package main

import (
	"log"
	"online-game/internal/organization"
	"online-game/internal/server"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	cfg := config.Load("organization-service")
	cfg.Port = 8009
	database, _ := db.New(&cfg.Database)
	defer database.Close()
	database.AutoMigration(&organization.Organization{}, &organization.OrganizationMember{})
	srv := server.New(cfg)
	srv.RegisterRoutes(organization.NewHandler().RegisterRoutes)
	go srv.Start()
	log.Printf("Organization Service started on %s", cfg.GetAddr())
	srv.WaitForShutdown()
}
