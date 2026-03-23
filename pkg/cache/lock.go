package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrLockFailed    = errors.New("failed to acquire lock")
	ErrLockNotHeld   = errors.New("lock not held")
	ErrLockReleased  = errors.New("lock already released")
)

// Lock represents a distributed lock
type Lock struct {
	client    *redis.Client
	key       string
	value     string
	ttl       time.Duration
	stopped   chan struct{}
}

// NewLock creates a new distributed lock
func NewLock(client *redis.Client, key string, ttl time.Duration) *Lock {
	return &Lock{
		client:  client,
		key:     "lock:" + key,
		value:   generateLockValue(),
		ttl:     ttl,
		stopped: make(chan struct{}),
	}
}

// generateLockValue generates a unique lock value
func generateLockValue() string {
	return time.Now().Format("20060102150405.000000000")
}

// TryLock attempts to acquire the lock once
func (l *Lock) TryLock(ctx context.Context) error {
	result, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return err
	}
	if !result {
		return ErrLockFailed
	}
	return nil
}

// Lock acquires the lock with retry
func (l *Lock) Lock(ctx context.Context, retryInterval time.Duration) error {
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err := l.TryLock(ctx)
			if err == nil {
				// Start auto-refresh goroutine
				go l.autoRefresh(ctx)
				return nil
			}
			if !errors.Is(err, ErrLockFailed) {
				return err
			}
		}
	}
}

// Unlock releases the lock
func (l *Lock) Unlock(ctx context.Context) error {
	// Stop auto-refresh
	close(l.stopped)

	// Use Lua script to ensure only lock holder can unlock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Result()
	if err != nil {
		return err
	}

	if result == int64(0) {
		return ErrLockNotHeld
	}

	return nil
}

// autoRefresh automatically refreshes the lock TTL
func (l *Lock) autoRefresh(ctx context.Context) {
	ticker := time.NewTicker(l.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopped:
			return
		case <-ticker.C:
			script := `
				if redis.call("get", KEYS[1]) == ARGV[1] then
					return redis.call("expire", KEYS[1], ARGV[2])
				else
					return 0
				end
			`

			l.client.Eval(ctx, script, []string{l.key}, l.value, int(l.ttl.Seconds()))
		}
	}
}

// Extend extends the lock TTL
func (l *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	result, err := l.client.Eval(ctx, script, []string{l.key}, l.value, int(ttl.Seconds())).Result()
	if err != nil {
		return err
	}

	if result == int64(0) {
		return ErrLockNotHeld
	}

	l.ttl = ttl
	return nil
}

// IsHeld checks if the lock is still held
func (l *Lock) IsHeld(ctx context.Context) (bool, error) {
	val, err := l.client.Get(ctx, l.key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == l.value, nil
}

// Mutex provides a simple mutex-like interface for distributed locking
type Mutex struct {
	client *redis.Client
	key    string
	ttl    time.Duration
}

// NewMutex creates a new distributed mutex
func NewMutex(client *redis.Client, key string, ttl time.Duration) *Mutex {
	return &Mutex{
		client: client,
		key:    "mutex:" + key,
		ttl:    ttl,
	}
}

// WithLock executes a function while holding the lock
func (m *Mutex) WithLock(ctx context.Context, fn func() error) error {
	lock := NewLock(m.client, m.key, m.ttl)

	if err := lock.Lock(ctx, 100*time.Millisecond); err != nil {
		return err
	}

	defer lock.Unlock(ctx)

	return fn()
}

// WithLockResult executes a function while holding the lock and returns the result
func (m *Mutex) WithLockResult(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	lock := NewLock(m.client, m.key, m.ttl)

	if err := lock.Lock(ctx, 100*time.Millisecond); err != nil {
		return nil, err
	}

	defer lock.Unlock(ctx)

	return fn()
}

// Semaphore provides a distributed semaphore for limiting concurrent access
type Semaphore struct {
	client *redis.Client
	key    string
	count  int
	ttl    time.Duration
}

// NewSemaphore creates a new distributed semaphore
func NewSemaphore(client *redis.Client, key string, count int, ttl time.Duration) *Semaphore {
	return &Semaphore{
		client: client,
		key:    "semaphore:" + key,
		count:  count,
		ttl:    ttl,
	}
}

// Acquire acquires a permit from the semaphore
func (s *Semaphore) Acquire(ctx context.Context) error {
	script := `
		local current = redis.call("zcard", KEYS[1])
		if current < tonumber(ARGV[1]) then
			redis.call("zadd", KEYS[1], ARGV[2], ARGV[3])
			redis.call("expire", KEYS[1], ARGV[4])
			return 1
		else
			-- Check if there are expired permits and clean them up
			local expiry = tonumber(ARGV[2]) - tonumber(ARGV[4]) * 2
			redis.call("zremrangebyscore", KEYS[1], "-inf", expiry)
			current = redis.call("zcard", KEYS[1])
			if current < tonumber(ARGV[1]) then
				redis.call("zadd", KEYS[1], ARGV[2], ARGV[3])
				redis.call("expire", KEYS[1], ARGV[4])
				return 1
			end
			return 0
		end
	`

	value := generateLockValue()
	score := float64(time.Now().UnixNano()) / 1e9

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			result, err := s.client.Eval(ctx, script, []string{s.key}, s.count, score, value, int(s.ttl.Seconds())).Result()
			if err != nil {
				return err
			}
			if result == int64(1) {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// TryAcquire tries to acquire a permit without waiting
func (s *Semaphore) TryAcquire(ctx context.Context) error {
	script := `
		local current = redis.call("zcard", KEYS[1])
		if current < tonumber(ARGV[1]) then
			redis.call("zadd", KEYS[1], ARGV[2], ARGV[3])
			redis.call("expire", KEYS[1], ARGV[4])
			return 1
		else
			-- Check if there are expired permits and clean them up
			local expiry = tonumber(ARGV[2]) - tonumber(ARGV[4]) * 2
			redis.call("zremrangebyscore", KEYS[1], "-inf", expiry)
			current = redis.call("zcard", KEYS[1])
			if current < tonumber(ARGV[1]) then
				redis.call("zadd", KEYS[1], ARGV[2], ARGV[3])
				redis.call("expire", KEYS[1], ARGV[4])
				return 1
			end
			return 0
		end
	`

	value := generateLockValue()
	score := float64(time.Now().UnixNano()) / 1e9

	result, err := s.client.Eval(ctx, script, []string{s.key}, s.count, score, value, int(s.ttl.Seconds())).Result()
	if err != nil {
		return err
	}
	if result == int64(0) {
		return ErrLockFailed
	}

	return nil
}

// Release releases a permit back to the semaphore
func (s *Semaphore) Release(ctx context.Context) error {
	value := generateLockValue()
	// Remove current permit and add a new one to indicate release
	script := `
		redis.call("zrem", KEYS[1], ARGV[1])
	`

	return s.client.Eval(ctx, script, []string{s.key}, value).Err()
}
