package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"online-game/internal/game"
	"online-game/internal/server"
	"online-game/pkg/actor"
	"online-game/pkg/config"
	"online-game/pkg/db"
	"online-game/pkg/websocket"
)

func main() {
	cfg := config.Load("game-service")
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	slog.Info("starting game-service", "port", cfg.Port)

	database, err := db.New(&cfg.Database)
	if err != nil {
		slog.Error("database error", "error", err)
		os.Exit(1)
	}
	defer db.Close(database)

	actorSystem := actor.NewActorSystem()
	wsGateway := websocket.NewGateway(websocket.DefaultGatewayConfig())

	gameSvc := game.NewService(database, actorSystem, wsGateway)
	gameSvc.Migrate()

	gameHandler := game.NewHandler(gameSvc)

	srv := server.New(&server.ServerConfig{Host: cfg.Host, Port: cfg.Port, Env: cfg.Env})
	gameHandler.RegisterRoutes(srv.Router().Group("/api/v1"))
	srv.Router().GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "game-service", "actors": actorSystem.Count()})
	})

	go srv.Start()
	srv.WaitForShutdown()

	actorSystem.Shutdown(10 * time.Second)
}
