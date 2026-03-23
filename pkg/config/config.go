// Package config provides configuration management for all services
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the configuration for a service
type Config struct {
	// Service configuration
	ServiceName string
	Environment string
	Host        string
	Port        int

	// Database configuration
	Database DatabaseConfig

	// Redis configuration
	Redis RedisConfig

	// Kafka configuration (optional)
	Kafka KafkaConfig

	// Logging configuration
	LogLevel string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers []string
	GroupID string
}

// Load loads configuration from environment variables
func Load(serviceName string) *Config {
	return &Config{
		ServiceName: serviceName,
		Environment: getEnv("ENV", "development"),
		Host:        getEnv("HOST", "0.0.0.0"),
		Port:        getEnvInt("PORT", 8000),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "192.168.3.78"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "6283213"),
			Database:        getEnv("DB_NAME", "game_platform_db"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE", 10),
			ConnMaxLifetime: time.Duration(getEnvInt("DB_CONN_LIFETIME", 300)) * time.Second,
		},

		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
			PoolSize: getEnvInt("REDIS_POOL_SIZE", 100),
		},

		Kafka: KafkaConfig{
			Brokers: []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
			GroupID: getEnv("KAFKA_GROUP_ID", serviceName),
		},
	}
}

// GetDSN returns the database connection string
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// GetRedisAddr returns the Redis address
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetAddr returns the service address
func (c *Config) GetAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// IsProduction returns true if running in production
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment returns true if running in development
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
