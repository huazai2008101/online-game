package cache

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Repository wraps database operations with caching
type Repository[T any] struct {
	db    *gorm.DB
	cache Cache
	ttl   time.Duration
}

// NewRepository creates a new cached repository
func NewRepository[T any](db *gorm.DB, cache Cache, ttl time.Duration) *Repository[T] {
	return &Repository[T]{
		db:    db,
		cache: cache,
		ttl:   ttl,
	}
}

// CacheKeyBuilder builds cache keys
type CacheKeyBuilder func(entity string, id interface{}) string

// DefaultKeyBuilder is the default key builder
func DefaultKeyBuilder(entity string, id interface{}) string {
	return fmt.Sprintf("%s:%v", entity, id)
}

// FindByID finds an entity by ID with caching
func (r *Repository[T]) FindByID(ctx context.Context, id uint, keyBuilder ...CacheKeyBuilder) (T, error) {
	var result T

	key := r.buildKey("entity", id, keyBuilder...)

	// Try cache first
	if err := r.cache.Get(ctx, key, &result); err == nil {
		return result, nil
	}

	// Cache miss, query database
	if err := r.db.WithContext(ctx).First(&result, id).Error; err != nil {
		return result, err
	}

	// Store in cache
	_ = r.cache.Set(ctx, key, result, r.ttl)

	return result, nil
}

// FindOne finds a single entity with custom conditions and caching
func (r *Repository[T]) FindOne(ctx context.Context, conditions map[string]interface{}, key string) (T, error) {
	var result T

	// Try cache first
	if err := r.cache.Get(ctx, key, &result); err == nil {
		return result, nil
	}

	// Cache miss, query database
	query := r.db.WithContext(ctx)
	for k, v := range conditions {
		query = query.Where(k+" = ?", v)
	}

	if err := query.First(&result).Error; err != nil {
		return result, err
	}

	// Store in cache
	_ = r.cache.Set(ctx, key, result, r.ttl)

	return result, nil
}

// FindMany finds multiple entities
func (r *Repository[T]) FindMany(ctx context.Context, conditions map[string]interface{}) ([]T, error) {
	var results []T

	query := r.db.WithContext(ctx)
	for k, v := range conditions {
		query = query.Where(k+" = ?", v)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

// Create creates a new entity
func (r *Repository[T]) Create(ctx context.Context, entity *T) error {
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return err
	}
	return nil
}

// Update updates an entity and invalidates cache
func (r *Repository[T]) Update(ctx context.Context, entity *T, id uint, keyBuilder ...CacheKeyBuilder) error {
	if err := r.db.WithContext(ctx).Model(entity).Where("id = ?", id).Updates(entity).Error; err != nil {
		return err
	}

	// Invalidate cache
	key := r.buildKey("entity", id, keyBuilder...)
	_ = r.cache.Delete(ctx, key)

	return nil
}

// Delete deletes an entity and invalidates cache
func (r *Repository[T]) Delete(ctx context.Context, id uint, keyBuilder ...CacheKeyBuilder) error {
	if err := r.db.WithContext(ctx).Delete(new(T), id).Error; err != nil {
		return err
	}

	// Invalidate cache
	key := r.buildKey("entity", id, keyBuilder...)
	_ = r.cache.Delete(ctx, key)

	return nil
}

// InvalidateCache invalidates cache for a specific key
func (r *Repository[T]) InvalidateCache(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.cache.Delete(ctx, keys...)
}

// SetCache manually sets cache
func (r *Repository[T]) SetCache(ctx context.Context, key string, value interface{}) error {
	return r.cache.Set(ctx, key, value, r.ttl)
}

// GetCache manually gets from cache
func (r *Repository[T]) GetCache(ctx context.Context, key string, dest interface{}) error {
	return r.cache.Get(ctx, key, dest)
}

// Exists checks if an entity exists (with cache)
func (r *Repository[T]) Exists(ctx context.Context, id uint, keyBuilder ...CacheKeyBuilder) (bool, error) {
	key := r.buildKey("entity", id, keyBuilder...)

	// Check cache first
	exists, err := r.cache.Exists(ctx, key)
	if err == nil && exists {
		return true, nil
	}

	// Check database
	var count int64
	if err := r.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// Count counts entities with conditions
func (r *Repository[T]) Count(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	var count int64

	query := r.db.WithContext(ctx).Model(new(T))
	for k, v := range conditions {
		query = query.Where(k+" = ?", v)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// ListWithPagination lists entities with pagination
func (r *Repository[T]) ListWithPagination(ctx context.Context, page, pageSize int, conditions map[string]interface{}) ([]T, int64, error) {
	var results []T
	var total int64

	query := r.db.WithContext(ctx).Model(new(T))
	for k, v := range conditions {
		query = query.Where(k+" = ?", v)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get page
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&results).Error; err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// BatchCreate creates multiple entities in batch
func (r *Repository[T]) BatchCreate(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).CreateInBatches(entities, 100).Error
}

// BatchUpdate updates multiple entities
func (r *Repository[T]) BatchUpdate(ctx context.Context, ids []uint, updates map[string]interface{}) error {
	if len(ids) == 0 || len(updates) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Model(new(T)).Where("id IN ?", ids).Updates(updates).Error
}

// BatchDelete deletes multiple entities and invalidates their cache
func (r *Repository[T]) BatchDelete(ctx context.Context, ids []uint, keyBuilder ...CacheKeyBuilder) error {
	if len(ids) == 0 {
		return nil
	}

	if err := r.db.WithContext(ctx).Delete(new(T), ids).Error; err != nil {
		return err
	}

	// Invalidate cache for all deleted entities
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = r.buildKey("entity", id, keyBuilder...)
	}
	_ = r.cache.Delete(ctx, keys...)

	return nil
}

// Transaction executes a function within a transaction
func (r *Repository[T]) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

// buildKey builds a cache key
func (r *Repository[T]) buildKey(entity string, id interface{}, keyBuilder ...CacheKeyBuilder) string {
	if len(keyBuilder) > 0 && keyBuilder[0] != nil {
		return keyBuilder[0](entity, id)
	}
	return DefaultKeyBuilder(entity, id)
}

// CachedQuery caches a custom query result
type CachedQuery[T any] struct {
	cache Cache
	ttl   time.Duration
}

// NewCachedQuery creates a new cached query helper
func NewCachedQuery[T any](cache Cache, ttl time.Duration) *CachedQuery[T] {
	return &CachedQuery[T]{
		cache: cache,
		ttl:   ttl,
	}
}

// Execute executes a query with caching
func (q *CachedQuery[T]) Execute(ctx context.Context, key string, fn func() (T, error)) (T, error) {
	var result T

	// Try cache first
	if err := q.cache.Get(ctx, key, &result); err == nil {
		return result, nil
	}

	// Execute query function
	data, err := fn()
	if err != nil {
		return result, err
	}

	// Store in cache
	_ = q.cache.Set(ctx, key, data, q.ttl)

	return data, nil
}

// ExecuteMany executes a query returning a slice with caching
func (q *CachedQuery[T]) ExecuteMany(ctx context.Context, key string, fn func() ([]T, error)) ([]T, error) {
	var result []T

	// Try cache first
	if err := q.cache.Get(ctx, key, &result); err == nil {
		return result, nil
	}

	// Execute query function
	data, err := fn()
	if err != nil {
		return nil, err
	}

	// Store in cache
	_ = q.cache.Set(ctx, key, data, q.ttl)

	return data, nil
}

// Invalidate invalidates cached query
func (q *CachedQuery[T]) Invalidate(ctx context.Context, key string) error {
	return q.cache.Delete(ctx, key)
}
