package config

import "os"

// Config holds all configuration for a service.
type Config struct {
	ServiceName string
	Env         string
	Host        string
	Port        string
	GRPCPort    string

	Database DatabaseConfig
	Redis    RedisConfig
	LogLevel string
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// Load reads configuration from environment variables.
func Load(serviceName string) *Config {
	return &Config{
		ServiceName: serviceName,
		Env:         envOr("ENV", "development"),
		Host:        envOr("HOST", "0.0.0.0"),
		Port:        envOr("PORT", defaultPort(serviceName)),
		GRPCPort:    envOr("GRPC_PORT", defaultGRPCPort(serviceName)),
		LogLevel:    envOr("LOG_LEVEL", "info"),
		Database: DatabaseConfig{
			Host:     envOr("DB_HOST", "localhost"),
			Port:     envOr("DB_PORT", "5432"),
			User:     envOr("DB_USER", "postgres"),
			Password: envOr("DB_PASSWORD", ""),
			DBName:   envOr("DB_NAME", serviceName),
			SSLMode:  envOr("DB_SSL_MODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     envOr("REDIS_HOST", "localhost"),
			Port:     envOr("REDIS_PORT", "6379"),
			Password: envOr("REDIS_PASSWORD", ""),
			DB:       0,
		},
	}
}

// IsDevelopment reports whether we're running in dev mode.
func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

// DSN returns the PostgreSQL connection string.
func (c *Config) DSN() string {
	return "host=" + c.Database.Host +
		" port=" + c.Database.Port +
		" user=" + c.Database.User +
		" password=" + c.Database.Password +
		" dbname=" + c.Database.DBName +
		" sslmode=" + c.Database.SSLMode
}

// RedisAddr returns the Redis address.
func (c *Config) RedisAddr() string {
	return c.Redis.Host + ":" + c.Redis.Port
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultPort(serviceName string) string {
	ports := map[string]string{
		"api-gateway":   "8080",
		"user-service":  "8001",
		"game-service":  "8002",
		"admin-service": "8003",
	}
	if p, ok := ports[serviceName]; ok {
		return p
	}
	return "8000"
}

func defaultGRPCPort(serviceName string) string {
	ports := map[string]string{
		"user-service":  "9001",
		"game-service":  "9002",
		"admin-service": "9003",
	}
	if p, ok := ports[serviceName]; ok {
		return p
	}
	return "9000"
}
