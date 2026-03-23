package db

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// PoolStats tracks connection pool statistics
type PoolStats struct {
	MaxOpenConnections int32         // Maximum number of open connections
	OpenConnections    int32         // Current number of open connections
	InUse              int32         // Number of connections currently in use
	Idle               int32         // Number of idle connections
	WaitCount          int64         // Total number of connections waited for
	WaitDuration       time.Duration // Total time waited for connections
	MaxIdleClosed      int64         // Total number of connections closed due to max idle
	MaxLifetimeClosed  int64         // Total number of connections closed due to max lifetime
	MaxIdleTimeClosed  int64         // Total number of connections closed due to max idle time
}

// ConnectionPool manages database connections with optimization
type ConnectionPool struct {
	db      *gorm.DB
	stats   PoolStats
	mu      sync.RWMutex
	config  *PoolConfig
	metrics *PoolMetrics
}

// PoolConfig defines connection pool configuration
type PoolConfig struct {
	MaxOpenConns    int           // Maximum number of open connections
	MaxIdleConns    int           // Maximum number of idle connections
	ConnMaxLifetime time.Duration // Maximum lifetime of a connection
	ConnMaxIdleTime time.Duration // Maximum idle time of a connection
	ConnMaxIdleTimeEnabled bool
	EnableMetrics    bool         // Enable metrics collection
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxOpenConns:    100,
		MaxIdleConns:    10,
		ConnMaxLifetime: 1 * time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
		ConnMaxIdleTimeEnabled: true,
		EnableMetrics:    true,
	}
}

// NewConnectionPool creates a new optimized connection pool
func NewConnectionPool(db *gorm.DB, config *PoolConfig) (*ConnectionPool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	if config.ConnMaxIdleTimeEnabled {
		sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	}

	pool := &ConnectionPool{
		db:     db,
		config: config,
		metrics: NewPoolMetrics(),
	}

	atomic.StoreInt32(&pool.stats.MaxOpenConnections, int32(config.MaxOpenConns))

	if config.EnableMetrics {
		go pool.collectMetrics()
	}

	return pool, nil
}

// DB returns the underlying GORM DB
func (p *ConnectionPool) DB() *gorm.DB {
	return p.db
}

// Stats returns current pool statistics
func (p *ConnectionPool) Stats() *PoolStats {
	sqlDB, err := p.db.DB()
	if err != nil {
		return &p.stats
	}

	dbStats := sqlDB.Stats()

	atomic.StoreInt32(&p.stats.OpenConnections, int32(dbStats.OpenConnections))
	atomic.StoreInt32(&p.stats.InUse, int32(dbStats.InUse))
	atomic.StoreInt32(&p.stats.Idle, int32(dbStats.Idle))
	atomic.StoreInt64(&p.stats.WaitCount, dbStats.WaitCount)
	atomic.StoreInt64(&p.stats.MaxIdleClosed, dbStats.MaxIdleClosed)
	atomic.StoreInt64(&p.stats.MaxLifetimeClosed, dbStats.MaxLifetimeClosed)
	atomic.StoreInt64(&p.stats.MaxIdleTimeClosed, dbStats.MaxIdleTimeClosed)

	return &p.stats
}

// collectMetrics periodically collects pool metrics
func (p *ConnectionPool) collectMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		p.Stats()
	}
}

// WithContext returns a new DB session with context
func (p *ConnectionPool) WithContext(ctx context.Context) *gorm.DB {
	return p.db.WithContext(ctx)
}

// Transaction executes a function within a transaction
func (p *ConnectionPool) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return p.db.WithContext(ctx).Transaction(fn)
}

// Close closes the database connection pool
func (p *ConnectionPool) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Ping verifies a connection to the database is still alive
func (p *ConnectionPool) Ping(ctx context.Context) error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Reconfigure reconfigures the connection pool
func (p *ConnectionPool) Reconfigure(config *PoolConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}

	if config.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(config.MaxOpenConns)
		atomic.StoreInt32(&p.stats.MaxOpenConnections, int32(config.MaxOpenConns))
	}

	if config.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	}

	if config.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	}

	if config.ConnMaxIdleTimeEnabled && config.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	}

	p.config = config
	return nil
}

// Config returns the current pool configuration
func (p *ConnectionPool) Config() *PoolConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// HealthCheck performs a health check on the connection pool
func (p *ConnectionPool) HealthCheck(ctx context.Context) error {
	stats := p.Stats()

	// Check if there are any connections available
	if stats.OpenConnections == 0 {
		return fmt.Errorf("no open connections")
	}

	// Check if too many connections are in use (80% threshold)
	usage := float64(stats.InUse) / float64(stats.MaxOpenConnections)
	if usage > 0.8 {
		return fmt.Errorf("high connection usage: %.2f%%", usage*100)
	}

	// Ping the database
	return p.Ping(ctx)
}

// PoolMetrics tracks detailed pool metrics
type PoolMetrics struct {
	queryCount      atomic.Int64
	slowQueries     atomic.Int64
	failedQueries   atomic.Int64
	totalQueryTime  atomic.Int64 // in nanoseconds
	maxQueryTime    atomic.Int64 // in nanoseconds
}

// NewPoolMetrics creates a new pool metrics tracker
func NewPoolMetrics() *PoolMetrics {
	return &PoolMetrics{}
}

// RecordQuery records a query execution
func (m *PoolMetrics) RecordQuery(duration time.Duration, slowThreshold time.Duration, err error) {
	m.queryCount.Add(1)
	m.totalQueryTime.Add(duration.Nanoseconds())

	// Update max query time
	for {
		max := m.maxQueryTime.Load()
		if duration.Nanoseconds() <= max {
			break
		}
		if m.maxQueryTime.CompareAndSwap(max, duration.Nanoseconds()) {
			break
		}
	}

	if duration > slowThreshold {
		m.slowQueries.Add(1)
	}

	if err != nil {
		m.failedQueries.Add(1)
	}
}

// GetMetrics returns current metrics
func (m *PoolMetrics) GetMetrics() map[string]int64 {
	return map[string]int64{
		"query_count":      m.queryCount.Load(),
		"slow_queries":     m.slowQueries.Load(),
		"failed_queries":   m.failedQueries.Load(),
		"total_query_time": m.totalQueryTime.Load(),
		"max_query_time":   m.maxQueryTime.Load(),
	}
}

// Reset resets all metrics
func (m *PoolMetrics) Reset() {
	m.queryCount.Store(0)
	m.slowQueries.Store(0)
	m.failedQueries.Store(0)
	m.totalQueryTime.Store(0)
	m.maxQueryTime.Store(0)
}

// QueryCallback returns a GORM callback for tracking queries
func (p *ConnectionPool) QueryCallback(slowThreshold time.Duration) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if !p.config.EnableMetrics {
			return
		}

		start := time.Now()
		duration := time.Since(start)

		err := db.Error
		p.metrics.RecordQuery(duration, slowThreshold, err)
	}
}

// MultiPool manages multiple database connection pools
type MultiPool struct {
	pools map[string]*ConnectionPool
	mu    sync.RWMutex
}

// NewMultiPool creates a new multi-pool manager
func NewMultiPool() *MultiPool {
	return &MultiPool{
		pools: make(map[string]*ConnectionPool),
	}
}

// Register registers a connection pool
func (mp *MultiPool) Register(name string, pool *ConnectionPool) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.pools[name] = pool
}

// Get retrieves a connection pool by name
func (mp *MultiPool) Get(name string) (*ConnectionPool, bool) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	pool, ok := mp.pools[name]
	return pool, ok
}

// GetAll returns all registered pools
func (mp *MultiPool) GetAll() map[string]*ConnectionPool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	result := make(map[string]*ConnectionPool, len(mp.pools))
	for name, pool := range mp.pools {
		result[name] = pool
	}
	return result
}

// HealthCheck performs health checks on all pools
func (mp *MultiPool) HealthCheck(ctx context.Context) map[string]error {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	results := make(map[string]error)
	for name, pool := range mp.pools {
		results[name] = pool.HealthCheck(ctx)
	}
	return results
}

// Close closes all connection pools
func (mp *MultiPool) Close(ctx context.Context) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	var firstErr error
	for name, pool := range mp.pools {
		if err := pool.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to close pool %s: %w", name, err)
		}
	}
	return firstErr
}

// Stats returns statistics for all pools
func (mp *MultiPool) Stats() map[string]*PoolStats {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	result := make(map[string]*PoolStats, len(mp.pools))
	for name, pool := range mp.pools {
		result[name] = pool.Stats()
	}
	return result
}
