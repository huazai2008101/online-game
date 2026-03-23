package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// InvalidationEvent represents a cache invalidation event
type InvalidationEvent struct {
	Key      string    `json:"key"`
	Pattern  string    `json:"pattern,omitempty"`
	Reason   string    `json:"reason,omitempty"`
	SenderID string    `json:"sender_id"`
	Time     time.Time `json:"time"`
}

// InvalidationStrategy defines the cache invalidation strategy
type InvalidationStrategy int

const (
	// InvalidationImmediate invalidates cache immediately
	InvalidationImmediate InvalidationStrategy = iota
	// InvalidationDelayed invalidates cache after a delay
	InvalidationDelayed
	// InvalidationVersioned uses versioning for cache entries
	InvalidationVersioned
)

// InvalidationPublisher publishes cache invalidation events
type InvalidationPublisher struct {
	client  *redis.Client
	channel string
	senderID string
}

// NewInvalidationPublisher creates a new invalidation publisher
func NewInvalidationPublisher(client *redis.Client, channel, senderID string) *InvalidationPublisher {
	return &InvalidationPublisher{
		client:   client,
		channel:  "cache:invalidate:" + channel,
		senderID: senderID,
	}
}

// Publish publishes an invalidation event for a specific key
func (p *InvalidationPublisher) Publish(ctx context.Context, key string, reason string) error {
	event := InvalidationEvent{
		Key:      key,
		Reason:   reason,
		SenderID: p.senderID,
		Time:     time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.client.Publish(ctx, p.channel, data).Err()
}

// PublishPattern publishes an invalidation event for a key pattern
func (p *InvalidationPublisher) PublishPattern(ctx context.Context, pattern string, reason string) error {
	event := InvalidationEvent{
		Pattern:  pattern,
		Reason:   reason,
		SenderID: p.senderID,
		Time:     time.Now(),
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.client.Publish(ctx, p.channel, data).Err()
}

// InvalidationSubscriber subscribes to cache invalidation events
type InvalidationSubscriber struct {
	client    *redis.Client
	channel   string
	senderID  string
	cache     Cache
	handlers  map[string]func(ctx context.Context, key string)
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewInvalidationSubscriber creates a new invalidation subscriber
func NewInvalidationSubscriber(client *redis.Client, channel, senderID string, cache Cache) *InvalidationSubscriber {
	ctx, cancel := context.WithCancel(context.Background())

	return &InvalidationSubscriber{
		client:   client,
		channel:  "cache:invalidate:" + channel,
		senderID: senderID,
		cache:    cache,
		handlers: make(map[string]func(ctx context.Context, key string)),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// RegisterHandler registers a handler for a specific key pattern
func (s *InvalidationSubscriber) RegisterHandler(pattern string, handler func(ctx context.Context, key string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[pattern] = handler
}

// Start starts the subscriber
func (s *InvalidationSubscriber) Start() error {
	pubsub := s.client.Subscribe(s.ctx, s.channel)
	_, err := pubsub.Receive(s.ctx)
	if err != nil {
		return err
	}

	s.wg.Add(1)
	go s.subscribe(pubsub)

	return nil
}

// Stop stops the subscriber
func (s *InvalidationSubscriber) Stop() {
	s.cancel()
	s.wg.Wait()
}

// subscribe listens for invalidation events
func (s *InvalidationSubscriber) subscribe(pubsub *redis.PubSub) {
	defer s.wg.Done()
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			var event InvalidationEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue
			}

			// Ignore events from this sender
			if event.SenderID == s.senderID {
				continue
			}

			s.handleEvent(s.ctx, event)
		}
	}
}

// handleEvent handles an invalidation event
func (s *InvalidationSubscriber) handleEvent(ctx context.Context, event InvalidationEvent) {
	if event.Key != "" {
		// Invalidate specific key
		_ = s.cache.Delete(ctx, event.Key)

		// Call registered handlers
		s.mu.RLock()
		for pattern, handler := range s.handlers {
			if matchPattern(pattern, event.Key) {
				go handler(ctx, event.Key)
			}
		}
		s.mu.RUnlock()
	}

	if event.Pattern != "" {
		// For pattern-based invalidation, we'd need to scan keys
		// This is more expensive and should be used sparingly
		s.invalidateByPattern(ctx, event.Pattern)
	}
}

// invalidateByPattern invalidates keys matching a pattern
func (s *InvalidationSubscriber) invalidateByPattern(ctx context.Context, pattern string) {
	// This requires scanning keys, which is expensive
	// In production, consider maintaining a set of cached keys per pattern
	iter := s.client.Scan(ctx, 0, pattern, 100).Iterator()
	keys := []string{}

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return
	}

	if len(keys) > 0 {
		_ = s.cache.Delete(ctx, keys...)
	}
}

// matchPattern checks if a key matches a pattern
func matchPattern(pattern, key string) bool {
	// Simple wildcard matching
	// For production, use a proper glob pattern library
	if pattern == "*" {
		return true
	}
	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	return pattern == key
}

// VersionedCache uses versioning for cache entries
type VersionedCache struct {
	cache    Cache
	versions map[string]int64
	mu       sync.RWMutex
	prefix   string
}

// NewVersionedCache creates a new versioned cache
func NewVersionedCache(cache Cache, prefix string) *VersionedCache {
	return &VersionedCache{
		cache:    cache,
		versions: make(map[string]int64),
		prefix:   prefix,
	}
}

// Get retrieves a value with version check
func (v *VersionedCache) Get(ctx context.Context, key string, dest interface{}) error {
	v.mu.RLock()
	version, exists := v.versions[key]
	v.mu.RUnlock()

	if !exists {
		return ErrCacheMiss
	}

	versionKey := v.buildVersionKey(key, version)
	return v.cache.Get(ctx, versionKey, dest)
}

// Set stores a value with a new version
func (v *VersionedCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	v.mu.Lock()
	version := v.versions[key] + 1
	v.versions[key] = version
	v.mu.Unlock()

	versionKey := v.buildVersionKey(key, version)
	return v.cache.Set(ctx, versionKey, value, ttl)
}

// Invalidate invalidates a cache entry by incrementing version
func (v *VersionedCache) Invalidate(ctx context.Context, key string) {
	v.mu.Lock()
	v.versions[key]++
	v.mu.Unlock()
}

// InvalidateAll invalidates all cache entries
func (v *VersionedCache) InvalidateAll(ctx context.Context) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for key := range v.versions {
		v.versions[key]++
	}
}

// GetVersion returns the current version of a key
func (v *VersionedCache) GetVersion(key string) int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.versions[key]
}

// buildVersionKey builds a versioned cache key
func (v *VersionedCache) buildVersionKey(key string, version int64) string {
	return fmt.Sprintf("%s:v%d:%s", v.prefix, version, key)
}

// DelayedInvalidation delays cache invalidation
type DelayedInvalidation struct {
	cache       Cache
	publisher   *InvalidationPublisher
	delays      map[string]time.Time
	mu          sync.Mutex
	ttl         time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewDelayedInvalidation creates a new delayed invalidation manager
func NewDelayedInvalidation(cache Cache, publisher *InvalidationPublisher, ttl time.Duration) *DelayedInvalidation {
	ctx, cancel := context.WithCancel(context.Background())

	di := &DelayedInvalidation{
		cache:     cache,
		publisher: publisher,
		delays:    make(map[string]time.Time),
		ttl:       ttl,
		ctx:       ctx,
		cancel:    cancel,
	}

	di.wg.Add(1)
	go di.process()

	return di
}

// Invalidate schedules a delayed invalidation
func (d *DelayedInvalidation) Invalidate(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.delays[key] = time.Now().Add(d.ttl)
}

// InvalidateNow invalidates immediately
func (d *DelayedInvalidation) InvalidateNow(ctx context.Context, key string) error {
	d.mu.Lock()
	delete(d.delays, key)
	d.mu.Unlock()

	return d.cache.Delete(ctx, key)
}

// Stop stops the delayed invalidation processor
func (d *DelayedInvalidation) Stop() {
	d.cancel()
	d.wg.Wait()
}

// process processes delayed invalidations
func (d *DelayedInvalidation) process() {
	defer d.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.checkInvalidations()
		}
	}
}

// checkInvalidations checks for and processes due invalidations
func (d *DelayedInvalidation) checkInvalidations() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for key, expiry := range d.delays {
		if now.After(expiry) {
			_ = d.cache.Delete(context.Background(), key)
			delete(d.delays, key)
		}
	}
}

// CacheInvalidator manages cache invalidation with different strategies
type CacheInvalidator struct {
	cache         Cache
	publisher     *InvalidationPublisher
	subscriber    *InvalidationSubscriber
	versioned     *VersionedCache
	delayed       *DelayedInvalidation
	strategy      InvalidationStrategy
	localKeys     map[string]string
	mu            sync.RWMutex
}

// NewCacheInvalidator creates a new cache invalidator
func NewCacheInvalidator(cache Cache, client *redis.Client, channel, senderID string, strategy InvalidationStrategy) *CacheInvalidator {
	publisher := NewInvalidationPublisher(client, channel, senderID)
	subscriber := NewInvalidationSubscriber(client, channel, senderID, cache)

	ci := &CacheInvalidator{
		cache:      cache,
		publisher:  publisher,
		subscriber: subscriber,
		strategy:   strategy,
		localKeys:  make(map[string]string),
	}

	if strategy == InvalidationVersioned {
		ci.versioned = NewVersionedCache(cache, channel)
	}

	if strategy == InvalidationDelayed {
		ci.delayed = NewDelayedInvalidation(cache, publisher, 5*time.Second)
	}

	return ci
}

// Start starts the invalidator
func (ci *CacheInvalidator) Start() error {
	if ci.subscriber != nil {
		if err := ci.subscriber.Start(); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops the invalidator
func (ci *CacheInvalidator) Stop() {
	if ci.subscriber != nil {
		ci.subscriber.Stop()
	}
	if ci.delayed != nil {
		ci.delayed.Stop()
	}
}

// Invalidate invalidates a cache key
func (ci *CacheInvalidator) Invalidate(ctx context.Context, key string) error {
	switch ci.strategy {
	case InvalidationImmediate:
		// Invalidate local cache
		_ = ci.cache.Delete(ctx, key)
		// Publish to other instances
		_ = ci.publisher.Publish(ctx, key, "immediate")

	case InvalidationDelayed:
		if ci.delayed != nil {
			ci.delayed.Invalidate(key)
		}

	case InvalidationVersioned:
		if ci.versioned != nil {
			ci.versioned.Invalidate(ctx, key)
		}
	}

	return nil
}

// InvalidatePattern invalidates keys matching a pattern
func (ci *CacheInvalidator) InvalidatePattern(ctx context.Context, pattern string) error {
	switch ci.strategy {
	case InvalidationImmediate:
		_ = ci.publisher.PublishPattern(ctx, pattern, "pattern")

	case InvalidationVersioned:
		ci.versioned.InvalidateAll(ctx)

	default:
		// For other strategies, use pattern-based invalidation
		_ = ci.publisher.PublishPattern(ctx, pattern, "pattern")
	}

	return nil
}

// RegisterHandler registers an invalidation handler
func (ci *CacheInvalidator) RegisterHandler(pattern string, handler func(ctx context.Context, key string)) {
	if ci.subscriber != nil {
		ci.subscriber.RegisterHandler(pattern, handler)
	}
}

// Get retrieves from cache with version support
func (ci *CacheInvalidator) Get(ctx context.Context, key string, dest interface{}) error {
	if ci.strategy == InvalidationVersioned && ci.versioned != nil {
		return ci.versioned.Get(ctx, key, dest)
	}
	return ci.cache.Get(ctx, key, dest)
}

// Set stores in cache with version support
func (ci *CacheInvalidator) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ci.strategy == InvalidationVersioned && ci.versioned != nil {
		return ci.versioned.Set(ctx, key, value, ttl)
	}
	return ci.cache.Set(ctx, key, value, ttl)
}

// AddLocalKey tracks a locally cached key
func (ci *CacheInvalidator) AddLocalKey(key string, tags ...string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	ci.localKeys[key] = key
}

// RemoveLocalKey stops tracking a local key
func (ci *CacheInvalidator) RemoveLocalKey(key string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	delete(ci.localKeys, key)
}

// InvalidateLocal invalidates all local cache
func (ci *CacheInvalidator) InvalidateLocal(ctx context.Context) error {
	ci.mu.Lock()
	keys := make([]string, 0, len(ci.localKeys))
	for key := range ci.localKeys {
		keys = append(keys, key)
	}
	ci.mu.Unlock()

	if len(keys) > 0 {
		return ci.cache.Delete(ctx, keys...)
	}

	return errors.New("no local keys to invalidate")
}
