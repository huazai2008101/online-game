package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"online-game/internal/gateway"
)

func main() {
	// Load configuration
	config := loadConfig()

	// Create gateway
	gw := gateway.NewGateway(config)

	// Register backends
	registerBackends(gw)

	// Start server in goroutine
	go func() {
		if err := gw.Start(); err != nil {
			log.Fatalf("Failed to start gateway: %v", err)
		}
	}()

	log.Printf("API Gateway started on port %d", config.Port)
	log.Println("Health check: http://localhost:8080/health")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gateway...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := gw.Shutdown(ctx); err != nil {
		log.Fatalf("Gateway shutdown failed: %v", err)
	}

	log.Println("Gateway stopped")
}

func loadConfig() *gateway.Config {
	return &gateway.Config{
		Port:            getEnvInt("PORT", 8080),
		Mode:            getEnv("MODE", "release"),
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		TracingEnabled:  true,
		MetricsEnabled:  true,
	}
}

func registerBackends(gw *gateway.Gateway) {
	backends := []*gateway.ServiceBackend{
		{
			Name:      "user-service",
			Prefix:    "/api/v1/users",
			TargetURL: getEnv("USER_SERVICE_URL", "http://localhost:8001"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "game-service",
			Prefix:    "/api/v1/games",
			TargetURL: getEnv("GAME_SERVICE_URL", "http://localhost:8002"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "payment-service",
			Prefix:    "/api/v1/payments",
			TargetURL: getEnv("PAYMENT_SERVICE_URL", "http://localhost:8003"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "player-service",
			Prefix:    "/api/v1/players",
			TargetURL: getEnv("PLAYER_SERVICE_URL", "http://localhost:8004"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "activity-service",
			Prefix:    "/api/v1/activities",
			TargetURL: getEnv("ACTIVITY_SERVICE_URL", "http://localhost:8005"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "guild-service",
			Prefix:    "/api/v1/guilds",
			TargetURL: getEnv("GUILD_SERVICE_URL", "http://localhost:8006"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "item-service",
			Prefix:    "/api/v1/items",
			TargetURL: getEnv("ITEM_SERVICE_URL", "http://localhost:8007"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "notification-service",
			Prefix:    "/api/v1/notifications",
			TargetURL: getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8008"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "organization-service",
			Prefix:    "/api/v1/organizations",
			TargetURL: getEnv("ORGANIZATION_SERVICE_URL", "http://localhost:8009"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "permission-service",
			Prefix:    "/api/v1/permissions",
			TargetURL: getEnv("PERMISSION_SERVICE_URL", "http://localhost:8010"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "id-service",
			Prefix:    "/api/v1/id",
			TargetURL: getEnv("ID_SERVICE_URL", "http://localhost:8011"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
		{
			Name:      "file-service",
			Prefix:    "/api/v1/files",
			TargetURL: getEnv("FILE_SERVICE_URL", "http://localhost:8012"),
			KeepAlive: 30 * time.Second,
			MaxIdle:   100,
		},
	}

	for _, backend := range backends {
		if err := gw.RegisterBackend(backend); err != nil {
			log.Printf("Warning: failed to register backend %s: %v", backend.Name, err)
		} else {
			log.Printf("Registered backend: %s -> %s", backend.Name, backend.TargetURL)
		}
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var intVal int
		if _, err := fmt.Sscanf(val, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultVal
}
