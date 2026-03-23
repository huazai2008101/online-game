package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrCacheMiss     = errors.New("cache miss")
	ErrInvalidType   = errors.New("invalid type")
	ErrNotConnected  = errors.New("redis not connected")
)

// Cache defines the cache interface
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Incr(ctx context.Context, key string) (int64, error)
	IncrBy(ctx context.Context, key string, value int64) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	GetString(ctx context.Context, key string) (string, error)
	SetString(ctx context.Context, key, value string, ttl time.Duration) error
	GetBytes(ctx context.Context, key string) ([]byte, error)
	SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// RedisCache implements Cache interface using Redis
type RedisCache struct {
	client *redis.Client
	prefix string
}

// New creates a new Redis cache instance
func New(client *redis.Client, prefix string) *RedisCache {
	return &RedisCache{
		client: client,
		prefix: prefix,
	}
}

// buildKey adds prefix to key
func (c *RedisCache) buildKey(key string) string {
	if c.prefix == "" {
		return key
	}
	return fmt.Sprintf("%s:%s", c.prefix, key)
}

// Get retrieves a value and unmarshals it into dest
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	if c.client == nil {
		return ErrNotConnected
	}

	val, err := c.client.Get(ctx, c.buildKey(key)).Result()
	if err != nil {
		if err == redis.Nil {
			return ErrCacheMiss
		}
		return err
	}

	return json.Unmarshal([]byte(val), dest)
}

// Set stores a value with TTL
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c.client == nil {
		return ErrNotConnected
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, c.buildKey(key), data, ttl).Err()
}

// Delete removes keys from cache
func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	if len(keys) == 0 {
		return nil
	}

	prefixedKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixedKeys[i] = c.buildKey(key)
	}

	return c.client.Del(ctx, prefixedKeys...).Err()
}

// Exists checks if a key exists
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	if c.client == nil {
		return false, ErrNotConnected
	}

	n, err := c.client.Exists(ctx, c.buildKey(key)).Result()
	return n > 0, err
}

// Expire sets a TTL on a key
func (c *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.Expire(ctx, c.buildKey(key), ttl).Err()
}

// Incr increments a counter
func (c *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	if c.client == nil {
		return 0, ErrNotConnected
	}

	return c.client.Incr(ctx, c.buildKey(key)).Result()
}

// IncrBy increments a counter by value
func (c *RedisCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	if c.client == nil {
		return 0, ErrNotConnected
	}

	return c.client.IncrBy(ctx, c.buildKey(key), value).Result()
}

// Decr decrements a counter
func (c *RedisCache) Decr(ctx context.Context, key string) (int64, error) {
	if c.client == nil {
		return 0, ErrNotConnected
	}

	return c.client.Decr(ctx, c.buildKey(key)).Result()
}

// GetString retrieves a string value
func (c *RedisCache) GetString(ctx context.Context, key string) (string, error) {
	if c.client == nil {
		return "", ErrNotConnected
	}

	val, err := c.client.Get(ctx, c.buildKey(key)).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

// SetString stores a string value
func (c *RedisCache) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.Set(ctx, c.buildKey(key), value, ttl).Err()
}

// GetBytes retrieves raw bytes
func (c *RedisCache) GetBytes(ctx context.Context, key string) ([]byte, error) {
	if c.client == nil {
		return nil, ErrNotConnected
	}

	val, err := c.client.Get(ctx, c.buildKey(key)).Bytes()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	}
	return val, err
}

// SetBytes stores raw bytes
func (c *RedisCache) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.Set(ctx, c.buildKey(key), value, ttl).Err()
}

// MultiGet retrieves multiple keys at once
func (c *RedisCache) MultiGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	if c.client == nil {
		return nil, ErrNotConnected
	}

	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	prefixedKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixedKeys[i] = c.buildKey(key)
	}

	vals, err := c.client.MGet(ctx, prefixedKeys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for i, val := range vals {
		if val != nil {
			if str, ok := val.(string); ok {
				result[keys[i]] = []byte(str)
			}
		}
	}

	return result, nil
}

// MultiSet stores multiple key-value pairs
func (c *RedisCache) MultiSet(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if c.client == nil {
		return ErrNotConnected
	}

	if len(items) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()
	for key, value := range items {
		data, err := json.Marshal(value)
		if err != nil {
			continue
		}
		pipe.Set(ctx, c.buildKey(key), data, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// Hash operations for complex data structures

// HGet retrieves a field from a hash
func (c *RedisCache) HGet(ctx context.Context, key, field string) (string, error) {
	if c.client == nil {
		return "", ErrNotConnected
	}

	val, err := c.client.HGet(ctx, c.buildKey(key), field).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

// HSet sets a field in a hash
func (c *RedisCache) HSet(ctx context.Context, key, field, value string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.HSet(ctx, c.buildKey(key), field, value).Err()
}

// HMGet retrieves multiple fields from a hash
func (c *RedisCache) HMGet(ctx context.Context, key string, fields ...string) (map[string]string, error) {
	if c.client == nil {
		return nil, ErrNotConnected
	}

	vals, err := c.client.HMGet(ctx, c.buildKey(key), fields...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for i, val := range vals {
		if val != nil {
			result[fields[i]] = val.(string)
		}
	}

	return result, nil
}

// HMSet sets multiple fields in a hash
func (c *RedisCache) HMSet(ctx context.Context, key string, fields map[string]string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	if len(fields) == 0 {
		return nil
	}

	data := make(map[string]interface{})
	for k, v := range fields {
		data[k] = v
	}

	return c.client.HMSet(ctx, c.buildKey(key), data).Err()
}

// HDel deletes fields from a hash
func (c *RedisCache) HDel(ctx context.Context, key string, fields ...string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	if len(fields) == 0 {
		return nil
	}

	return c.client.HDel(ctx, c.buildKey(key), fields...).Err()
}

// HGetAll retrieves all fields from a hash
func (c *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	if c.client == nil {
		return nil, ErrNotConnected
	}

	vals, err := c.client.HGetAll(ctx, c.buildKey(key)).Result()
	if err != nil {
		return nil, err
	}

	return vals, nil
}

// List operations

// LPush pushes values to the left of a list
func (c *RedisCache) LPush(ctx context.Context, key string, values ...string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.LPush(ctx, c.buildKey(key), values).Err()
}

// RPush pushes values to the right of a list
func (c *RedisCache) RPush(ctx context.Context, key string, values ...string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.RPush(ctx, c.buildKey(key), values).Err()
}

// LPop pops a value from the left of a list
func (c *RedisCache) LPop(ctx context.Context, key string) (string, error) {
	if c.client == nil {
		return "", ErrNotConnected
	}

	val, err := c.client.LPop(ctx, c.buildKey(key)).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

// RPop pops a value from the right of a list
func (c *RedisCache) RPop(ctx context.Context, key string) (string, error) {
	if c.client == nil {
		return "", ErrNotConnected
	}

	val, err := c.client.RPop(ctx, c.buildKey(key)).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

// LRange retrieves a range of elements from a list
func (c *RedisCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if c.client == nil {
		return nil, ErrNotConnected
	}

	return c.client.LRange(ctx, c.buildKey(key), start, stop).Result()
}

// Set operations for unique collections

// SAdd adds members to a set
func (c *RedisCache) SAdd(ctx context.Context, key string, members ...string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.SAdd(ctx, c.buildKey(key), members).Err()
}

// SRem removes members from a set
func (c *RedisCache) SRem(ctx context.Context, key string, members ...string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.SRem(ctx, c.buildKey(key), members).Err()
}

// SMembers retrieves all members of a set
func (c *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	if c.client == nil {
		return nil, ErrNotConnected
	}

	return c.client.SMembers(ctx, c.buildKey(key)).Result()
}

// SIsMember checks if a member is in a set
func (c *RedisCache) SIsMember(ctx context.Context, key, member string) (bool, error) {
	if c.client == nil {
		return false, ErrNotConnected
	}

	return c.client.SIsMember(ctx, c.buildKey(key), member).Result()
}

// Sorted set operations

// ZAdd adds members to a sorted set
func (c *RedisCache) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.ZAdd(ctx, c.buildKey(key), members...).Err()
}

// ZRem removes members from a sorted set
func (c *RedisCache) ZRem(ctx context.Context, key string, members ...string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.ZRem(ctx, c.buildKey(key), members).Err()
}

// ZRange retrieves a range of members from a sorted set by rank
func (c *RedisCache) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	if c.client == nil {
		return nil, ErrNotConnected
	}

	return c.client.ZRange(ctx, c.buildKey(key), start, stop).Result()
}

// ZRangeByScore retrieves members from a sorted set by score range
func (c *RedisCache) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error) {
	if c.client == nil {
		return nil, ErrNotConnected
	}

	return c.client.ZRangeByScore(ctx, c.buildKey(key), opt).Result()
}

// ZIncrBy increments the score of a member in a sorted set
func (c *RedisCache) ZIncrBy(ctx context.Context, key string, increment float64, member string) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.ZIncrBy(ctx, c.buildKey(key), increment, member).Err()
}

// ZScore retrieves the score of a member in a sorted set
func (c *RedisCache) ZScore(ctx context.Context, key, member string) (float64, error) {
	if c.client == nil {
		return 0, ErrNotConnected
	}

	return c.client.ZScore(ctx, c.buildKey(key), member).Result()
}

// ZRank retrieves the rank of a member in a sorted set (ascending)
func (c *RedisCache) ZRank(ctx context.Context, key, member string) (int64, error) {
	if c.client == nil {
		return 0, ErrNotConnected
	}

	return c.client.ZRank(ctx, c.buildKey(key), member).Result()
}

// ZRevRank retrieves the rank of a member in a sorted set (descending)
func (c *RedisCache) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	if c.client == nil {
		return 0, ErrNotConnected
	}

	return c.client.ZRevRank(ctx, c.buildKey(key), member).Result()
}

// Flush clears all cache entries with this prefix
func (c *RedisCache) Flush(ctx context.Context) error {
	if c.client == nil {
		return ErrNotConnected
	}

	if c.prefix == "" {
		return c.client.FlushDB(ctx).Err()
	}

	iter := c.client.Scan(ctx, 0, c.buildKey("*"), 100).Iterator()
	keys := []string{}

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return err
	}

	if len(keys) > 0 {
		return c.client.Del(ctx, keys...).Err()
	}

	return nil
}

// Ping checks if Redis is connected
func (c *RedisCache) Ping(ctx context.Context) error {
	if c.client == nil {
		return ErrNotConnected
	}

	return c.client.Ping(ctx).Err()
}
