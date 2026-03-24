# 缓存系统详细实现

**文档版本:** v1.0
**创建时间:** 2026-03-24

---

## 目录

1. [设计概述](#1-设计概述)
2. [Redis 缓存](#2-redis-缓存)
3. [本地缓存](#3-本地缓存)
4. [多级缓存](#4-多级缓存)
5. [缓存策略](#5-缓存策略)
6. [使用示例](#6-使用示例)

---

## 1. 设计概述

### 1.1 设计目标

1. **高性能**: 快速读写，低延迟
2. **高可用**: 支持故障转移
3. **一致性**: 保证数据一致性
4. **可扩展**: 支持水平扩展

### 1.2 架构设计

```
                    ┌─────────────────┐
                    │   Application   │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │   Cache Manager │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
   ┌────▼─────┐       ┌─────▼─────┐       ┌────▼─────┐
   │   L1     │       │    L2     │       │   L3     │
   │ Local    │       │  Redis   │       │ Memcached│
   │ Cache    │       │  Cache   │       │   Cache  │
   └──────────┘       └───────────┘       └──────────┘
```

---

## 2. Redis 缓存

### 2.1 Redis 客户端实现

```go
// pkg/cache/redis.go
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient Redis缓存客户端
type RedisClient struct {
	client *redis.Client
	config RedisConfig
	mu     sync.RWMutex
	stats  CacheStats
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
}

// DefaultRedisConfig 默认配置
var DefaultRedisConfig = RedisConfig{
	Addr:         "localhost:6379",
	DB:           0,
	PoolSize:     100,
	MinIdleConns: 10,
	DialTimeout:  5 * time.Second,
	ReadTimeout:  3 * time.Second,
	WriteTimeout: 3 * time.Second,
	PoolTimeout:  4 * time.Second,
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(config RedisConfig) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolTimeout:  config.PoolTimeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{
		client: client,
		config: config,
	}, nil
}

// Get 获取缓存
func (r *RedisClient) Get(ctx context.Context, key string) ([]byte, error) {
	r.stats.Increment(StatsGet)

	val, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			r.stats.Increment(StatsMiss)
			return nil, ErrCacheNotFound
		}
		r.stats.Increment(StatsError)
		return nil, err
	}

	r.stats.Increment(StatsHit)
	return val, nil
}

// Set 设置缓存
func (r *RedisClient) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	r.stats.Increment(StatsSet)

	if err := r.client.Set(ctx, key, value, expiration).Err(); err != nil {
		r.stats.Increment(StatsError)
		return err
	}

	return nil
}

// Delete 删除缓存
func (r *RedisClient) Delete(ctx context.Context, key string) error {
	r.stats.Increment(StatsDelete)

	if err := r.client.Del(ctx, key).Err(); err != nil {
		r.stats.Increment(StatsError)
		return err
	}

	return nil
}

// Exists 检查key是否存在
func (r *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Expire 设置过期时间
func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// TTL 获取剩余过期时间
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// GetJSON 获取JSON值
func (r *RedisClient) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := r.Get(ctx, key)
	if err != nil {
		return err
	}

	return json.Unmarshal(val, dest)
}

// SetJSON 设置JSON值
func (r *RedisClient) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return r.Set(ctx, key, data, expiration)
}

// MGet 批量获取
func (r *RedisClient) MGet(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for i, val := range vals {
		if val != nil {
			result[keys[i]] = []byte(val)
		}
	}

	return result, nil
}

// MSet 批量设置
func (r *RedisClient) MSet(ctx context.Context, pairs map[string][]byte, expiration time.Duration) error {
	if len(pairs) == 0 {
		return nil
	}

	// 使用Pipeline批量设置
	pipe := r.client.Pipeline()

	for key, val := range pairs {
		if expiration > 0 {
			pipe.Set(ctx, key, val, expiration)
		} else {
			pipe.Set(ctx, key, val, 0)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

// Incr 递增
func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// IncrBy 递增指定值
func (r *RedisClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.IncrBy(ctx, key, value).Result()
}

// Decr 递减
func (r *RedisClient) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.Decr(ctx, key).Result()
}

// DecrBy 递减指定值
func (r *RedisClient) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.DecrBy(ctx, key, value).Result()
}

// HGet 获取哈希字段
func (r *RedisClient) HGet(ctx context.Context, key, field string) ([]byte, error) {
	val, err := r.client.HGet(ctx, key, field).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrCacheNotFound
		}
		return nil, err
	}
	return val, nil
}

// HSet 设置哈希字段
func (r *RedisClient) HSet(ctx context.Context, key, field string, value []byte) error {
	return r.client.HSet(ctx, key, field, value).Err()
}

// HGetAll 获取所有哈希字段
func (r *RedisClient) HGetAll(ctx context.Context, key string) (map[string][]byte, error) {
	vals, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for i := 0; i < len(vals); i += 2 {
		if vals[i] != nil && vals[i+1] != nil {
			result[string(vals[i].([]byte))] = vals[i+1].([]byte)
		}
	}

	return result, nil
}

// HDel 删除哈希字段
func (r *RedisClient) HDel(ctx context.Context, key string, fields ...string) error {
	return r.client.HDel(ctx, key, fields...).Err()
}

// HIncrBy 哈希字段递增
func (r *RedisClient) HIncrBy(ctx context.Context, key, field string, value int64) (int64, error) {
	return r.client.HIncrBy(ctx, key, field, value).Result()
}

// LPush 列表左推入
func (r *RedisClient) LPush(ctx context.Context, key string, values ...[]byte) error {
	args := make([]interface{}, len(values))
	for i, v := range values {
		args[i] = v
	}
	return r.client.LPush(ctx, key, args...).Err()
}

// RPush 列表右推入
func (r *RedisClient) RPush(ctx context.Context, key string, values ...[]byte) error {
	args := make([]interface{}, len(values))
	for i, v := range values {
		args[i] = v
	}
	return r.client.RPush(ctx, key, args...).Err()
}

// LPop 列表左弹出
func (r *RedisClient) LPop(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.LPop(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrCacheNotFound
		}
		return nil, err
	}
	return val, nil
}

// RPop 列表右弹出
func (r *RedisClient) RPop(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.RPop(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrCacheNotFound
		}
		return nil, err
	}
	return val, nil
}

// LRange 获取列表范围
func (r *RedisClient) LRange(ctx context.Context, key string, start, stop int64) ([][]byte, error) {
	vals, err := r.client.LRange(ctx, key, start, stop).Result()
	if err != nil {
		return nil, err
	}

	result := make([][]byte, len(vals))
	for i, val := range vals {
		result[i] = []byte(val)
	}

	return result, nil
}

// LLen 获取列表长度
func (r *RedisClient) LLen(ctx context.Context, key string) (int64, error) {
	return r.client.LLen(ctx, key).Result()
}

// SAdd 集合添加
func (r *RedisClient) SAdd(ctx context.Context, key string, members ...[]byte) error {
	args := make([]interface{}, len(members))
	for i, m := range members {
		args[i] = m
	}
	return r.client.SAdd(ctx, key, args...).Err()
}

// SRem 集合移除
func (r *RedisClient) SRem(ctx context.Context, key string, members ...[]byte) error {
	args := make([]interface{}, len(members))
	for i, m := range members {
		args[i] = m
	}
	return r.client.SRem(ctx, key, args...).Err()
}

// SMembers 获取所有集合成员
func (r *RedisClient) SMembers(ctx context.Context, key string) ([][]byte, error) {
	vals, err := r.client.SMembers(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	result := make([][]byte, len(vals))
	for i, val := range vals {
		result[i] = []byte(val)
	}

	return result, nil
}

// SIsMember 检查是否是集合成员
func (r *RedisClient) SIsMember(ctx context.Context, key string, member []byte) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

// ZAdd 有序集合添加
func (r *RedisClient) ZAdd(ctx context.Context, key string, score float64, member []byte) error {
	return r.client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// ZRange 有序集合范围查询（分数升序）
func (r *RedisClient) ZRange(ctx context.Context, key string, start, stop int64) (map[string]float64, error) {
	vals, err := r.client.ZRangeWithScores(ctx, key, start, stop).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]float64)
	for _, val := range vals {
		result[val.Member.(string)] = val.Score
	}

	return result, nil
}

// ZRevRange 有序集合范围查询（分数降序）
func (r *RedisClient) ZRevRange(ctx context.Context, key string, start, stop int64) (map[string]float64, error) {
	vals, err := r.client.ZRevRangeWithScores(ctx, key, start, stop).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]float64)
	for _, val := range vals {
		result[val.Member.(string)] = val.Score
	}

	return result, nil
}

// ZIncrBy 有序集合分数递增
func (r *RedisClient) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	return r.client.ZIncrBy(ctx, key, increment, member).Result()
}

// ZRank 获取排名（升序）
func (r *RedisClient) ZRank(ctx context.Context, key string, member string) (int64, error) {
	return r.client.ZRank(ctx, key, member).Result()
}

// ZRevRank 获取排名（降序）
func (r *RedisClient) ZRevRank(ctx context.Context, key string, member string) (int64, error) {
	return r.client.ZRevRank(ctx, key, member).Result()
}

// ZScore 获取分数
func (r *RedisClient) ZScore(ctx context.Context, key, member string) (float64, error) {
	return r.client.ZScore(ctx, key, member).Result()
}

// Publish 发布消息
func (r *RedisClient) Publish(ctx context.Context, channel string, message []byte) error {
	r.stats.Increment(StatsPublish)

	return r.client.Publish(ctx, channel, message).Err()
}

// Subscribe 订阅消息
func (r *RedisClient) Subscribe(ctx context.Context, channel string) <-chan *redis.Message {
	pubsub := r.client.Subscribe(ctx, channel)
	return pubsub.Channel()
}

// PSubscribe 模式订阅
func (r *RedisClient) PSubscribe(ctx context.Context, pattern string) <-chan *redis.Message {
	pubsub := r.client.PSubscribe(ctx, pattern)
	return pubsub.Channel()
}

// Eval 执行Lua脚本
func (r *RedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return r.client.Eval(ctx, script, keys, args...).Result()
}

// EvalSha 执行已缓存的Lua脚本
func (r *RedisClient) EvalSha(ctx context.Context, sha string, keys []string, args ...interface{}) (interface{}, error) {
	return r.client.EvalSha(ctx, sha, keys, args...).Result()
}

// ScriptLoad 加载Lua脚本
func (r *RedisClient) ScriptLoad(ctx context.Context, script string) (string, error) {
	return r.client.ScriptLoad(ctx, script).Result()
}

// FlushDB 清空数据库
func (r *RedisClient) FlushDB(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}

// FlushAll 清空所有数据库
func (r *RedisClient) FlushAll(ctx context.Context) error {
	return r.client.FlushAll(ctx).Err()
}

// Pipeline 返回Pipeline
func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// TxPipeline 返回事务Pipeline
func (r *RedisClient) TxPipeline() redis.Pipeliner {
	return r.client.TxPipeline()
}

// Close 关闭连接
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Stats 返回统计信息
func (r *RedisClient) Stats() CacheStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}
```

---

## 3. 本地缓存

### 3.1 内存缓存实现

```go
// pkg/cache/memory.go
package cache

import (
	"container/list"
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryCache 内存缓存
type MemoryCache struct {
	items    map[string]*cacheItem
	mu       sync.RWMutex
	lruList  *list.List
	lruMap   map[string]*list.Element
	maxSize  int
	stats   CacheStats
	closed  atomic.Bool
}

// cacheItem 缓存项
type cacheItem struct {
	key       string
	value     []byte
	expiration time.Time
	createdAt time.Time
	accessAt  atomic.Time // 最后访问时间
	hits      atomic.Int64 // 命中次数
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(maxSize int) *MemoryCache {
	if maxSize <= 0 {
		maxSize = 10000
	}

	mc := &MemoryCache{
		items:   make(map[string]*cacheItem),
		lruList: list.New(),
		lruMap:  make(map[string]*list.Element),
		maxSize: maxSize,
	}

	// 启动清理协程
	go mc.cleanupLoop()

	return mc
}

// Get 获取缓存
func (m *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	if m.closed.Load() {
		return nil, ErrCacheClosed
	}

	m.mu.RLock()
	item, exists := m.items[key]
	m.mu.RUnlock()

	if !exists {
		m.stats.Increment(StatsMiss)
		return nil, ErrCacheNotFound
	}

	// 检查过期
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		m.mu.Lock()
		delete(m.items, key)
		if elem, ok := m.lruMap[key]; ok {
			m.lruList.Remove(elem)
			delete(m.lruMap, key)
		}
		m.mu.Unlock()
		m.stats.Increment(StatsMiss)
		return nil, ErrCacheNotFound
	}

	// 更新访问时间和命中次数
	item.accessAt.Store(time.Now())
	item.hits.Add(1)

	// 更新LRU
	m.mu.Lock()
	if elem, ok := m.lruMap[key]; ok {
		m.lruList.MoveToFront(elem)
	}
	m.mu.Unlock()

	m.stats.Increment(StatsHit)
	return item.value, nil
}

// Set 设置缓存
func (m *MemoryCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	if m.closed.Load() {
		return ErrCacheClosed
	}

	var exp time.Time
	if expiration > 0 {
		exp = time.Now().Add(expiration)
	}

	item := &cacheItem{
		key:       key,
		value:     value,
		expiration: exp,
		createdAt: time.Now(),
	}
	item.accessAt.Store(time.Now())

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否需要淘汰
	if len(m.items) >= m.maxSize {
		if elem := m.lruList.Back(); elem != nil {
			m.removeItem(elem.Value.(*cacheItem).key)
		}
	}

	// 添加或更新
	if oldItem, exists := m.items[key]; exists {
		m.stats.Increment(StatsUpdate)
		if atomic.LoadInt64(&oldItem.hits) > 0 {
			atomic.AddInt64(&m.stats.Hit, -atomic.LoadInt64(&oldItem.hits))
		}
	} else {
		m.stats.Increment(StatsSet)
	}

	m.items[key] = item

	// 更新LRU
	if elem, ok := m.lruMap[key]; ok {
		m.lruList.MoveToFront(elem)
	} else {
		elem := m.lruList.PushFront(key)
		m.lruMap[key] = elem
	}

	return nil
}

// Delete 删除缓存
func (m *MemoryCache) Delete(ctx context.Context, key string) error {
	if m.closed.Load() {
		return ErrCacheClosed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.items[key]; !exists {
		return ErrCacheNotFound
	}

	m.removeItem(key)
	m.stats.Increment(StatsDelete)
	return nil
}

// removeItem 移除缓存项（已加锁）
func (m *MemoryCache) removeItem(key string) {
	delete(m.items, key)
	if elem, ok := m.lruMap[key]; ok {
		m.lruList.Remove(elem)
		delete(m.lruMap, key)
	}
}

// Exists 检查key是否存在
func (m *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	if m.closed.Load() {
		return false, ErrCacheClosed
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return false, nil
	}

	// 检查过期
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		return false, nil
	}

	return true, nil
}

// Expire 设置过期时间
func (m *MemoryCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if m.closed.Load() {
		return ErrCacheClosed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.items[key]
	if !exists {
		return ErrCacheNotFound
	}

	item.expiration = time.Now().Add(expiration)
	return nil
}

// TTL 获取剩余过期时间
func (m *MemoryCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	if m.closed.Load() {
		return 0, ErrCacheClosed
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return 0, ErrCacheNotFound
	}

	if item.expiration.IsZero() {
		return -1, nil // 永不过期
	}

	ttl := time.Until(item.expiration)
	if ttl < 0 {
		return 0, nil // 已过期
	}

	return ttl, nil
}

// GetJSON 获取JSON值
func (m *MemoryCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := m.Get(ctx, key)
	if err != nil {
		return err
	}

	return json.Unmarshal(val, dest)
}

// SetJSON 设置JSON值
func (m *MemoryCache) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return m.Set(ctx, key, data, expiration)
}

// MGet 批量获取
func (m *MemoryCache) MGet(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if m.closed.Load() {
		return nil, ErrCacheClosed
	}

	result := make(map[string][]byte)

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, key := range keys {
		if item, exists := m.items[key]; exists {
			if item.expiration.IsZero() || time.Now().Before(item.expiration) {
				result[key] = item.value
			}
		}
	}

	return result, nil
}

// MSet 批量设置
func (m *MemoryCache) MSet(ctx context.Context, pairs map[string][]byte, expiration time.Duration) error {
	if m.closed.Load() {
		return ErrCacheClosed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for key, value := range pairs {
		var exp time.Time
		if expiration > 0 {
			exp = time.Now().Add(expiration)
		}

		item := &cacheItem{
			key:       key,
			value:     value,
			expiration: exp,
			createdAt: time.Now(),
		}
		item.accessAt.Store(time.Now())

		m.items[key] = item

		if elem, ok := m.lruMap[key]; ok {
			m.lruList.MoveToFront(elem)
		} else {
			elem := m.lruList.PushFront(key)
			m.lruMap[key] = elem
		}
	}

	return nil
}

// Incr 递增
func (m *MemoryCache) Incr(ctx context.Context, key string) (int64, error) {
	return m.incrBy(ctx, key, 1)
}

// IncrBy 递增指定值
func (m *MemoryCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return m.incrBy(ctx, key, value)
}

func (m *MemoryCache) incrBy(ctx context.Context, key string, delta int64) (int64, error) {
	if m.closed.Load() {
		return 0, ErrCacheClosed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.items[key]
	if !exists {
		// 创建新项
		newItem := &cacheItem{
			key:   key,
			value: []byte("0"),
		}
		newItem.accessAt.Store(time.Now())

		m.items[key] = newItem

		elem := m.lruList.PushFront(key)
		m.lruMap[key] = elem

		return delta, nil
	}

	// 解析当前值
	var current int64
	if _, err := fmt.Sscanf(string(item.value), "%d", &current); err != nil {
		current = 0
	}

	newValue := current + delta
	item.value = []byte(fmt.Sprintf("%d", newValue))
	item.accessAt.Store(time.Now())

	// 更新LRU
	if elem, ok := m.lruMap[key]; ok {
		m.lruList.MoveToFront(elem)
	}

	return newValue, nil
}

// Decr 递减
func (m *MemoryCache) Decr(ctx context.Context, key string) (int64, error) {
	return m.incrBy(ctx, key, -1)
}

// DecrBy 递减指定值
func (m *MemoryCache) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	return m.incrBy(ctx, key, -value)
}

// Clear 清空缓存
func (m *MemoryCache) Clear(ctx context.Context) error {
	if m.closed.Load() {
		return ErrCacheClosed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = make(map[string]*cacheItem)
	m.lruList = list.New()
	m.lruMap = make(map[string]*list.Element)

	return nil
}

// Items 返回所有缓存项
func (m *MemoryCache) Items(ctx context.Context) (map[string]*CacheItemInfo, error) {
	if m.closed.Load() {
		return nil, ErrCacheClosed
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*CacheItemInfo)

	for key, item := range m.items {
		// 检查过期
		if !item.expiration.IsZero() && time.Now().After(item.expiration) {
			continue
		}

		var ttl time.Duration
		if !item.expiration.IsZero() {
			ttl = time.Until(item.expiration)
		}

		result[key] = &CacheItemInfo{
			Key:       key,
			Size:      len(item.value),
			CreatedAt: item.createdAt,
			ExpiresAt: item.expiration,
			TTL:       ttl,
			Hits:      item.hits.Load(),
		}
	}

	return result, nil
}

// Size 返回缓存大小
func (m *MemoryCache) Size(ctx context.Context) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// Close 关闭缓存
func (m *MemoryCache) Close() error {
	if !m.closed.CompareAndSwap(false, true) {
		m.Clear(context.Background())
	}
	return nil
}

// Stats 返回统计信息
func (m *MemoryCache) Stats() CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats
	stats.Items = len(m.items)

	return stats
}

// cleanupLoop 清理过期项
func (m *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

// cleanup 清理过期项
func (m *MemoryCache) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, item := range m.items {
		if !item.expiration.IsZero() && now.After(item.expiration) {
			m.removeItem(key)
		}
	}
}

// CacheItemInfo 缓存项信息
type CacheItemInfo struct {
	Key       string
	Size      int
	CreatedAt time.Time
	ExpiresAt time.Time
	TTL       time.Duration
	Hits      int64
}
```

---

## 4. 多级缓存

### 4.1 多级缓存实现

```go
// pkg/cache/multilevel.go
package cache

import (
	"context"
	"sync"
	"time"
)

// MultiLevelCache 多级缓存
type MultiLevelCache struct {
	l1 Cache // 本地缓存
	l2 Cache // Redis缓存
	l3 Cache // 其他缓存

	mu    sync.RWMutex
	stats CacheStats
}

// NewMultiLevelCache 创建多级缓存
func NewMultiLevelCache(l1, l2 Cache) *MultiLevelCache {
	return &MultiLevelCache{
		l1: l1,
		l2: l2,
	}
}

// Get 获取缓存（逐级查找）
func (m *MultiLevelCache) Get(ctx context.Context, key string) ([]byte, error) {
	// L1: 本地缓存
	val, err := m.l1.Get(ctx, key)
	if err == nil {
		m.stats.Increment(StatsL1Hit)
		return val, nil
	}

	// L2: Redis缓存
	if m.l2 != nil {
		val, err = m.l2.Get(ctx, key)
		if err == nil {
			// 回填L1
			_ = m.l1.Set(ctx, key, val, 5*time.Minute)
			m.stats.Increment(StatsL2Hit)
			return val, nil
		}
	}

	m.stats.Increment(StatsMiss)
	return nil, ErrCacheNotFound
}

// Set 设置缓存（写入所有级别）
func (m *MultiLevelCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	// 并行写入所有级别
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// 写入L1
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := m.l1.Set(ctx, key, value, expiration); err != nil {
			errChan <- err
		}
	}()

	// 写入L2
	if m.l2 != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.l2.Set(ctx, key, value, expiration); err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// 检查错误
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}

	m.stats.Increment(StatsSet)
	return nil
}

// Delete 删除缓存（从所有级别删除）
func (m *MultiLevelCache) Delete(ctx context.Context, key string) error {
	// 并行删除所有级别
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// 从L1删除
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := m.l1.Delete(ctx, key); err != nil && err != ErrCacheNotFound {
			errChan <- err
		}
	}()

	// 从L2删除
	if m.l2 != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.l2.Delete(ctx, key); err != nil && err != ErrCacheNotFound {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// 检查错误
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}

	m.stats.Increment(StatsDelete)
	return nil
}

// GetJSON 获取JSON值
func (m *MultiLevelCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := m.Get(ctx, key)
	if err != nil {
		return err
	}

	return json.Unmarshal(val, dest)
}

// SetJSON 设置JSON值
func (m *MultiLevelCache) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return m.Set(ctx, key, data, expiration)
}

// Exists 检查key是否存在
func (m *MultiLevelCache) Exists(ctx context.Context, key string) (bool, error) {
	// 先检查L1
	exists, err := m.l1.Exists(ctx, key)
	if err == nil && exists {
		return true, nil
	}

	// 检查L2
	if m.l2 != nil {
		exists, err = m.l2.Exists(ctx, key)
		if err == nil && exists {
			return true, nil
		}
	}

	return false, nil
}

// Stats 返回统计信息
func (m *MultiLevelCache) Stats() CacheStats {
	// TODO: 聚合各级缓存的统计
	return m.stats
}

// Close 关闭所有缓存
func (m *MultiLevelCache) Close() error {
	var errs []error

	if m.l1 != nil {
		if err := m.l1.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if m.l2 != nil {
		if err := m.l2.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if m.l3 != nil {
		if err := m.l3.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}
```

---

## 5. 缓存策略

### 5.1 缓存策略实现

```go
// pkg/cache/strategy.go
package cache

import (
	"context"
	"sync"
	"time"
)

// Cache 缓存接口
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
}

// Loader 数据加载器
type Loader func(ctx context.Context, key string) ([]byte, error)

// CacheAside Cache-Aside模式
type CacheAside struct {
	cache  Cache
	loader Loader
}

// NewCacheAside 创建Cache-Aside缓存
func NewCacheAside(cache Cache, loader Loader) *CacheAside {
	return &CacheAside{
		cache:  cache,
		loader: loader,
	}
}

// Get 获取数据（缓存未命中时加载）
func (c *CacheAside) Get(ctx context.Context, key string) ([]byte, error) {
	// 先尝试从缓存获取
	val, err := c.cache.Get(ctx, key)
	if err == nil {
		return val, nil
	}

	if err != ErrCacheNotFound {
		return nil, err
	}

	// 缓存未命中，加载数据
	val, err = c.loader(ctx, key)
	if err != nil {
		return nil, err
	}

	// 写入缓存
	_ = c.cache.Set(ctx, key, val, 5*time.Minute)

	return val, nil
}

// WriteThrough Write-Through模式
type WriteThrough struct {
	cache  Cache
	writer func(ctx context.Context, key string, value []byte) error
}

// NewWriteThrough 创建Write-Through缓存
func NewWriteThrough(cache Cache, writer func(ctx context.Context, key string, value []byte) error) *WriteThrough {
	return &WriteThrough{
		cache:  cache,
		writer: writer,
	}
}

// Set 设置数据（同时写入缓存和存储）
func (w *WriteThrough) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	// 先写存储
	if err := w.writer(ctx, key, value); err != nil {
		return err
	}

	// 再写缓存
	return w.cache.Set(ctx, key, value, expiration)
}

// WriteBack Write-Back模式
type WriteBack struct {
	cache      Cache
	writer     func(ctx context.Context, key string, value []byte) error
	mu         sync.Mutex
	pending    map[string]*writeBackItem
	flushTimer *time.Timer
}

type writeBackItem struct {
	key        string
	value      []byte
	expiration time.Duration
}

// NewWriteBack 创建Write-Back缓存
func NewWriteBack(cache Cache, writer func(ctx context.Context, key string, value []byte) error, flushInterval time.Duration) *WriteBack {
	wb := &WriteBack{
		cache:   cache,
		writer:  writer,
		pending: make(map[string]*writeBackItem),
	}

	// 定时刷新
	wb.flushTimer = time.AfterFunc(flushInterval, func() {
		_ = wb.flush()
	})

	return wb
}

// Set 设置数据（只写缓存，延迟写存储）
func (w *WriteBack) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	// 写入缓存
	if err := w.cache.Set(ctx, key, value, expiration); err != nil {
		return err
	}

	// 加入待刷新队列
	w.mu.Lock()
	w.pending[key] = &writeBackItem{
		key:        key,
		value:      value,
		expiration: expiration,
	}
	w.mu.Unlock()

	return nil
}

// flush 刷新所有待写入数据
func (w *WriteBack) flush() error {
	w.mu.Lock()
	pending := w.pending
	w.pending = make(map[string]*writeBackItem)
	w.mu.Unlock()

	for _, item := range pending {
		_ = w.writer(context.Background(), item.key, item.value)
	}

	// 重置定时器
	w.flushTimer.Reset(time.Minute)

	return nil
}

// RefreshAhead 预刷新模式
type RefreshAhead struct {
	cache      Cache
	loader     Loader
	refreshTTL time.Duration
}

// NewRefreshAhead 创建预刷新缓存
func NewRefreshAhead(cache Cache, loader Loader, refreshTTL time.Duration) *RefreshAhead {
	ra := &RefreshAhead{
		cache:      cache,
		loader:     loader,
		refreshTTL: refreshTTL,
	}

	// 启动后台刷新
	go ra.refreshLoop()

	return ra
}

// Get 获取数据（接近过期时异步刷新）
func (r *RefreshAhead) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.cache.Get(ctx, key)
	if err == nil {
		// 检查是否需要预刷新
		ttl, _ := r.cache.TTL(ctx, key)
		if ttl < r.refreshTTL && ttl > 0 {
			go r.refresh(key)
		}
		return val, nil
	}

	// 缓存未命中，加载数据
	val, err = r.loader(ctx, key)
	if err != nil {
		return nil, err
	}

	// 写入缓存
	_ = r.cache.Set(ctx, key, val, 5*time.Minute)

	return val, nil
}

// refresh 刷新数据
func (r *RefreshAhead) refresh(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	val, err := r.loader(ctx, key)
	if err != nil {
		return err
	}

	return r.cache.Set(ctx, key, val, 5*time.Minute)
}

// refreshLoop 刷新循环
func (r *RefreshAhead) refreshLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// TODO: 扫描需要刷新的key
	}
}
```

---

## 6. 使用示例

### 6.1 基本使用

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/your-org/online-game/pkg/cache"
)

func main() {
	// 创建Redis客户端
	redisClient, err := cache.NewRedisClient(cache.RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if err != nil {
		panic(err)
	}
	defer redisClient.Close()

	ctx := context.Background()

	// 设置缓存
	err = redisClient.Set(ctx, "user:1001", []byte(`{"name": "Alice"}`), 10*time.Minute)
	if err != nil {
		panic(err)
	}

	// 获取缓存
	val, err := redisClient.Get(ctx, "user:1001")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Value: %s\n", string(val))

	// 使用JSON
	var user User
	err = redisClient.GetJSON(ctx, "user:1001", &user)
	if err != nil {
		panic(err)
	}

	fmt.Printf("User: %+v\n", user)
}

type User struct {
	Name string `json:"name"`
}
```

### 6.2 多级缓存使用

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/your-org/online-game/pkg/cache"
)

func main() {
	// 创建多级缓存
	l1 := cache.NewMemoryCache(10000)
	l2, _ := cache.NewRedisClient(cache.RedisConfig{
		Addr: "localhost:6379",
	})

	cache := cache.NewMultiLevelCache(l1, l2)

	ctx := context.Background()

	// 设置缓存
	err := cache.Set(ctx, "config:game", []byte(`{"maxPlayers": 100}`), time.Hour)
	if err != nil {
		panic(err)
	}

	// 获取缓存（优先从本地获取）
	val, err := cache.Get(ctx, "config:game")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Value: %s\n", string(val))
}
```
