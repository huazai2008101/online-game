package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"online-game/pkg/config"
)

// NewClient creates a new Redis client from config.
func NewClient(cfg *config.RedisConfig) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:     cfg.Host + ":" + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}

// Health checks the Redis connection.
func Health(client *goredis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return client.Ping(ctx).Err()
}

// Close gracefully closes the Redis client.
func Close(client *goredis.Client) error {
	if client == nil {
		return nil
	}
	return client.Close()
}

// Cache helpers

// SetJSON caches a value with TTL.
func SetJSON(ctx context.Context, client *goredis.Client, key string, value any, ttl time.Duration) error {
	if client == nil {
		return nil
	}
	return client.Set(ctx, key, value, ttl).Err()
}

// GetCached retrieves a cached string value.
func GetCached(ctx context.Context, client *goredis.Client, key string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("no redis client")
	}
	return client.Get(ctx, key).Result()
}

// Delete deletes keys matching the pattern.
func Delete(ctx context.Context, client *goredis.Client, keys ...string) error {
	if client == nil {
		return nil
	}
	return client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists.
func Exists(ctx context.Context, client *goredis.Client, key string) (bool, error) {
	if client == nil {
		return false, nil
	}
	n, err := client.Exists(ctx, key).Result()
	return n > 0, err
}
