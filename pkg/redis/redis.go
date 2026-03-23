// Package redis provides Redis client and utilities
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"online-game/pkg/config"
)

// Client wraps the Redis client
type Client struct {
	*redis.Client
}

// New creates a new Redis client
func New(cfg *config.RedisConfig) (*Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{Client: client}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.Client.Close()
}

// Get retrieves a value from Redis
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.Client.Get(ctx, key).Result()
}

// Set stores a value in Redis
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.Client.Set(ctx, key, value, expiration).Err()
}

// Delete removes a key from Redis
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	return c.Client.Del(ctx, keys...).Err()
}

// Exists checks if keys exist
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.Client.Exists(ctx, keys...).Result()
}

// Expire sets an expiration time for a key
func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.Client.Expire(ctx, key, expiration).Err()
}

// TTL returns the remaining time to live of a key
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.Client.TTL(ctx, key).Result()
}

// HGet retrieves a field from a hash
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	return c.Client.HGet(ctx, key, field).Result()
}

// HSet sets a field in a hash
func (c *Client) HSet(ctx context.Context, key string, values ...interface{}) error {
	return c.Client.HSet(ctx, key, values...).Err()
}

// HGetAll retrieves all fields and values from a hash
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.Client.HGetAll(ctx, key).Result()
}

// HDel removes fields from a hash
func (c *Client) HDel(ctx context.Context, key string, fields ...string) error {
	return c.Client.HDel(ctx, key, fields...).Err()
}

// LPush pushes values to the left of a list
func (c *Client) LPush(ctx context.Context, key string, values ...interface{}) error {
	return c.Client.LPush(ctx, key, values...).Err()
}

// RPush pushes values to the right of a list
func (c *Client) RPush(ctx context.Context, key string, values ...interface{}) error {
	return c.Client.RPush(ctx, key, values...).Err()
}

// LPop pops a value from the left of a list
func (c *Client) LPop(ctx context.Context, key string) (string, error) {
	return c.Client.LPop(ctx, key).Result()
}

// RPop pops a value from the right of a list
func (c *Client) RPop(ctx context.Context, key string) (string, error) {
	return c.Client.RPop(ctx, key).Result()
}

// LRange retrieves a range of elements from a list
func (c *Client) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.Client.LRange(ctx, key, start, stop).Result()
}

// SAdd adds members to a set
func (c *Client) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return c.Client.SAdd(ctx, key, members...).Err()
}

// SMembers retrieves all members of a set
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.Client.SMembers(ctx, key).Result()
}

// SIsMember checks if a member is in a set
func (c *Client) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return c.Client.SIsMember(ctx, key, member).Result()
}

// SRem removes members from a set
func (c *Client) SRem(ctx context.Context, key string, members ...interface{}) error {
	return c.Client.SRem(ctx, key, members...).Err()
}

// ZAdd adds members to a sorted set
func (c *Client) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	return c.Client.ZAdd(ctx, key, members...).Err()
}

// ZRange retrieves a range of members from a sorted set by rank
func (c *Client) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.Client.ZRange(ctx, key, start, stop).Result()
}

// ZRangeByScore retrieves a range of members from a sorted set by score
func (c *Client) ZRangeByScore(ctx context.Context, key string, min, max string, offset, count int64) ([]string, error) {
	return c.Client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    min,
		Max:    max,
		Offset: offset,
		Count:  count,
	}).Result()
}

// ZRem removes members from a sorted set
func (c *Client) ZRem(ctx context.Context, key string, members ...interface{}) error {
	return c.Client.ZRem(ctx, key, members...).Err()
}

// Incr increments a key's value
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.Client.Incr(ctx, key).Result()
}

// Decr decrements a key's value
func (c *Client) Decr(ctx context.Context, key string) (int64, error) {
	return c.Client.Decr(ctx, key).Result()
}
