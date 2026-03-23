// Package engine implements a dual-engine system for game logic execution
// It supports both JavaScript (V8 via otto) and WebAssembly engines
// with intelligent switching based on performance metrics
package engine

import (
	"context"
	"sync"
	"time"
)

// EngineType represents the type of game engine
type EngineType int

const (
	EngineJS EngineType = iota
	EngineWASM
)

// String returns the string representation of the engine type
func (e EngineType) String() string {
	switch e {
	case EngineJS:
		return "javascript"
	case EngineWASM:
		return "wasm"
	default:
		return "unknown"
	}
}

// GameEngine is the interface for all game engines
type GameEngine interface {
	// Init initializes the engine with game ID and initial state
	Init(ctx context.Context, gameID string, state any) error

	// LoadScript loads a game script of the specified type
	LoadScript(scriptType EngineType, script []byte) error

	// Execute executes a game logic method
	Execute(ctx context.Context, method string, args any) (any, error)

	// SwitchEngine switches to a different engine type
	SwitchEngine(engineType EngineType) error

	// Cleanup cleans up engine resources
	Cleanup() error

	// CurrentEngine returns the current engine type
	CurrentEngine() EngineType

	// Stats returns engine performance statistics
	Stats() *EngineStats
}

// EngineStats holds performance statistics for an engine
type EngineStats struct {
	EngineType      EngineType `json:"engine_type"`
	ExecuteCount    int64      `json:"execute_count"`
	ExecuteTime     int64      `json:"execute_time_ns"` // Total execution time in nanoseconds
	ErrorCount      int64      `json:"error_count"`
	MemoryUsage     int64      `json:"memory_usage_bytes"`
	CacheHitCount   int64      `json:"cache_hit_count"`
	CacheMissCount  int64      `json:"cache_miss_count"`
	LastExecuteTime time.Time  `json:"last_execute_time"`
	mu              sync.RWMutex
}

// GetAverageExecuteTime returns the average execution time in nanoseconds
func (s *EngineStats) GetAverageExecuteTime() int64 {
	if s.ExecuteCount == 0 {
		return 0
	}
	return s.ExecuteTime / s.ExecuteCount
}

// GetCacheHitRate returns the cache hit rate (0-1)
func (s *EngineStats) GetCacheHitRate() float64 {
	total := s.CacheHitCount + s.CacheMissCount
	if total == 0 {
		return 0
	}
	return float64(s.CacheHitCount) / float64(total)
}

// PerformanceData holds performance data for engine selection
type PerformanceData struct {
	AverageFPS  float64       `json:"average_fps"`
	ExecuteTime time.Duration `json:"execute_time"`
	MemoryUsage int64         `json:"memory_usage"`
	Complexity  int           `json:"complexity"`
	ErrorCount  int64         `json:"error_count"`
}

// EngineSelector selects the best engine based on performance data
type EngineSelector struct {
	performanceThreshold int           // FPS threshold for performance
	complexityThreshold  int           // Complexity threshold
	memoryThreshold      int64         // Memory threshold in bytes
	selectionHistory     []EngineType  // History of selections
	maxHistory           int           // Max history size
}

// NewEngineSelector creates a new engine selector
func NewEngineSelector() *EngineSelector {
	return &EngineSelector{
		performanceThreshold: 60,
		complexityThreshold:  100,
		memoryThreshold:      100 * 1024 * 1024, // 100MB
		selectionHistory:     make([]EngineType, 0, 10),
		maxHistory:           10,
	}
}

// SelectEngine selects the best engine based on performance data
func (s *EngineSelector) SelectEngine(perfData *PerformanceData) EngineType {
	// Performance evaluation - if performance is good, stay with current
	if perfData.AverageFPS >= float64(s.performanceThreshold) {
		if len(s.selectionHistory) > 0 {
			return s.selectionHistory[len(s.selectionHistory)-1]
		}
		return EngineJS
	}

	// Low performance - switch to WASM
	if perfData.AverageFPS < 30 {
		s.recordSelection(EngineWASM)
		return EngineWASM
	}

	// High complexity - use WASM
	if perfData.Complexity >= s.complexityThreshold {
		s.recordSelection(EngineWASM)
		return EngineWASM
	}

	// High memory usage - stay with JS
	if perfData.MemoryUsage > s.memoryThreshold {
		s.recordSelection(EngineJS)
		return EngineJS
	}

	// Default to JavaScript
	s.recordSelection(EngineJS)
	return EngineJS
}

// recordSelection records an engine selection
func (s *EngineSelector) recordSelection(engineType EngineType) {
	s.selectionHistory = append(s.selectionHistory, engineType)
	if len(s.selectionHistory) > s.maxHistory {
		s.selectionHistory = s.selectionHistory[1:]
	}
}

// GetSelectionHistory returns the selection history
func (s *EngineSelector) GetSelectionHistory() []EngineType {
	return s.selectionHistory
}

// SetPerformanceThreshold sets the performance threshold
func (s *EngineSelector) SetPerformanceThreshold(threshold int) {
	s.performanceThreshold = threshold
}

// SetComplexityThreshold sets the complexity threshold
func (s *EngineSelector) SetComplexityThreshold(threshold int) {
	s.complexityThreshold = threshold
}
