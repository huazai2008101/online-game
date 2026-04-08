package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Load_Defaults(t *testing.T) {
	// Clear env vars to avoid interference
	envKeys := []string{
		"SERVICE_NAME", "PORT", "GRPC_PORT", "LOG_LEVEL",
		"DB_HOST", "DB_PORT", "DB_NAME", "DB_USER", "DB_PASSWORD",
		"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "REDIS_DB",
		"JWT_SECRET", "JWT_EXPIRY_HOURS",
	}
	for _, k := range envKeys {
		os.Unsetenv(k)
	}

	cfg := Load("user-service")

	assert.Equal(t, "user-service", cfg.ServiceName)
	assert.Equal(t, "8001", cfg.Port)
	assert.Equal(t, "9001", cfg.GRPCPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "change-me-in-production", cfg.JWTSecret)
	assert.Equal(t, 72, cfg.JWTExpiryHours)

	// DB defaults
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "5432", cfg.Database.Port)
	assert.Equal(t, "user-service", cfg.Database.DBName)

	// Redis defaults
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, "6379", cfg.Redis.Port)
}

func TestConfig_Load_EnvOverride(t *testing.T) {
	os.Setenv("PORT", "9999")
	os.Setenv("JWT_SECRET", "my-custom-secret")
	os.Setenv("JWT_EXPIRY_HOURS", "48")
	os.Setenv("DB_HOST", "db.example.com")
	os.Setenv("REDIS_HOST", "cache.example.com")

	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("JWT_EXPIRY_HOURS")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("REDIS_HOST")
	}()

	cfg := Load("user-service")

	assert.Equal(t, "9999", cfg.Port)
	assert.Equal(t, "my-custom-secret", cfg.JWTSecret)
	assert.Equal(t, 48, cfg.JWTExpiryHours)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, "cache.example.com", cfg.Redis.Host)
}

func TestConfig_DefaultPorts(t *testing.T) {
	tests := []struct {
		service string
		port    string
		grpc    string
	}{
		{"api-gateway", "8080", "9000"},
		{"user-service", "8001", "9001"},
		{"game-service", "8002", "9002"},
		{"admin-service", "8003", "9003"},
	}

	for _, tt := range tests {
		t.Run(tt.service, func(t *testing.T) {
			cfg := Load(tt.service)
			assert.Equal(t, tt.port, cfg.Port)
			assert.Equal(t, tt.grpc, cfg.GRPCPort)
		})
	}
}

func TestConfig_RedisAddr(t *testing.T) {
	cfg := Config{
		Redis: RedisConfig{
			Host: "redis-host",
			Port: "6380",
		},
	}
	assert.Equal(t, "redis-host:6380", cfg.RedisAddr())
}
