// Database Migration Tool
package main

import (
	"fmt"
	"log"
	"os"

	"online-game/internal/game"
	"online-game/internal/notification"
	"online-game/internal/payment"
	"online-game/internal/player"
	"online-game/internal/user"
	"online-game/pkg/config"
	"online-game/pkg/db"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate <command>")
		fmt.Println("Commands:")
		fmt.Println("  up   - Run all migrations")
		fmt.Println("  down - Rollback all migrations")
		fmt.Println("  status - Check migration status")
		os.Exit(1)
	}

	command := os.Args[1]

	// Load database configuration
	cfg := &config.DatabaseConfig{
		Host:         getEnv("DB_HOST", "192.168.3.78"),
		Port:         getEnvInt("DB_PORT", 5432),
		User:         getEnv("DB_USER", "postgres"),
		Password:     getEnv("DB_PASSWORD", "6283213"),
		SSLMode:      getEnv("DB_SSLMODE", "disable"),
		MaxOpenConns: 25,
		MaxIdleConns: 10,
	}

	switch command {
	case "up":
		runMigrations(cfg)
	case "down":
		fmt.Println("Rollback not implemented yet")
	case "status":
		checkStatus(cfg)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func runMigrations(cfg *config.DatabaseConfig) {
	log.Println("Starting database migrations...")

	// Migrate game_platform_db
	log.Println("Migrating game_platform_db...")
	cfg.Database = "game_platform_db"
	migratePlatformDB(cfg)

	// Migrate game_core_db
	log.Println("Migrating game_core_db...")
	cfg.Database = "game_core_db"
	migrateCoreDB(cfg)

	// Migrate game_payment_db
	log.Println("Migrating game_payment_db...")
	cfg.Database = "game_payment_db"
	migratePaymentDB(cfg)

	// Migrate game_notification_db
	log.Println("Migrating game_notification_db...")
	cfg.Database = "game_notification_db"
	migrateNotificationDB(cfg)

	log.Println("All migrations completed successfully!")
}

func migratePlatformDB(cfg *config.DatabaseConfig) {
	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := database.AutoMigration(
		&user.User{},
		&user.UserProfile{},
		&user.Friend{},
		&user.UserSession{},
	); err != nil {
		log.Fatalf("Failed to migrate platform database: %v", err)
	}

	log.Println("  - users table")
	log.Println("  - user_profiles table")
	log.Println("  - friends table")
	log.Println("  - user_sessions table")
}

func migrateCoreDB(cfg *config.DatabaseConfig) {
	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := database.AutoMigration(
		&game.Game{},
		&game.GameVersion{},
		&game.GameRoom{},
		&game.GameSession{},
		&player.Player{},
		&player.PlayerStats{},
	); err != nil {
		log.Fatalf("Failed to migrate core database: %v", err)
	}

	log.Println("  - games table")
	log.Println("  - game_versions table")
	log.Println("  - game_rooms table")
	log.Println("  - game_sessions table")
	log.Println("  - players table")
	log.Println("  - player_stats table")
}

func migratePaymentDB(cfg *config.DatabaseConfig) {
	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := database.AutoMigration(
		&payment.Order{},
		&payment.Score{},
		&payment.ScoreLog{},
	); err != nil {
		log.Fatalf("Failed to migrate payment database: %v", err)
	}

	log.Println("  - orders table")
	log.Println("  - scores table")
	log.Println("  - score_logs table")
}

func migrateNotificationDB(cfg *config.DatabaseConfig) {
	database, err := db.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := database.AutoMigration(
		&notification.Notification{},
		&notification.Message{},
	); err != nil {
		log.Fatalf("Failed to migrate notification database: %v", err)
	}

	log.Println("  - notifications table")
	log.Println("  - messages table")
}

func checkStatus(cfg *config.DatabaseConfig) {
	log.Println("Checking migration status...")

	databases := []struct {
		name   string
		tables []string
	}{
		{"game_platform_db", []string{"users", "user_profiles", "friends", "user_sessions"}},
		{"game_core_db", []string{"games", "game_versions", "game_rooms", "game_sessions", "players", "player_stats"}},
		{"game_payment_db", []string{"orders", "scores", "score_logs"}},
		{"game_notification_db", []string{"notifications", "messages"}},
	}

	for _, dbInfo := range databases {
		cfg.Database = dbInfo.name
		database, err := db.New(cfg)
		if err != nil {
			log.Printf("  %s: FAILED - %v", dbInfo.name, err)
			continue
		}

		sqlDB, err := database.DB.DB()
		if err != nil {
			log.Printf("  %s: FAILED - %v", dbInfo.name, err)
			continue
		}

		rows, err := sqlDB.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'")
		if err != nil {
			log.Printf("  %s: ERROR - %v", dbInfo.name, err)
			database.Close()
			continue
		}

		var existingTables []string
		for rows.Next() {
			var table string
			rows.Scan(&table)
			existingTables = append(existingTables, table)
		}
		rows.Close()

		allExist := true
		for _, table := range dbInfo.tables {
			found := false
			for _, existing := range existingTables {
				if existing == table {
					found = true
					break
				}
			}
			if !found {
				allExist = false
				break
			}
		}

		if allExist {
			log.Printf("  %s: OK", dbInfo.name)
		} else {
			log.Printf("  %s: INCOMPLETE", dbInfo.name)
		}

		database.Close()
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
