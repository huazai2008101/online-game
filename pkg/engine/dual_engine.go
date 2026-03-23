package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DualEngine manages both JavaScript and WebAssembly engines
// with intelligent switching based on performance metrics
type DualEngine struct {
	jsEngine    *JSEngine
	wasmEngine  *WASMEngine
	currentType EngineType
	selector    *EngineSelector
	perfMonitor *PerformanceMonitor
	mu          sync.RWMutex
	gameID      string
}

// NewDualEngine creates a new dual engine system
func NewDualEngine(gameID string) (*DualEngine, error) {
	jsEngine := NewJSEngine()
	wasmEngine, err := NewWASMEngine()
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM engine: %w", err)
	}

	return &DualEngine{
		jsEngine:    jsEngine,
		wasmEngine:  wasmEngine,
		currentType: EngineJS, // Start with JS for faster development
		selector:    NewEngineSelector(),
		perfMonitor: NewPerformanceMonitor(),
		gameID:      gameID,
	}, nil
}

// Init initializes the dual engine system
func (d *DualEngine) Init(ctx context.Context, gameID string, state any) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.gameID = gameID

	// Initialize both engines
	if err := d.jsEngine.Init(ctx, gameID, state); err != nil {
		return fmt.Errorf("JS engine init failed: %w", err)
	}

	if err := d.wasmEngine.Init(ctx, gameID, state); err != nil {
		return fmt.Errorf("WASM engine init failed: %w", err)
	}

	return nil
}

// LoadScript loads a script for the specified engine type
func (d *DualEngine) LoadScript(scriptType EngineType, script []byte) error {
	switch scriptType {
	case EngineJS:
		return d.jsEngine.LoadScript(scriptType, script)
	case EngineWASM:
		return d.wasmEngine.LoadScript(scriptType, script)
	default:
		return fmt.Errorf("unknown script type: %v", scriptType)
	}
}

// Execute executes a method on the current engine
func (d *DualEngine) Execute(ctx context.Context, method string, args any) (any, error) {
	d.mu.RLock()
	currentEngine := d.currentType
	d.mu.RUnlock()

	start := time.Now()

	var result any
	var err error

	// Execute on current engine
	switch currentEngine {
	case EngineJS:
		result, err = d.jsEngine.Execute(ctx, method, args)
	case EngineWASM:
		result, err = d.wasmEngine.Execute(ctx, method, args)
	default:
		return nil, fmt.Errorf("unknown engine type: %v", currentEngine)
	}

	// Record performance
	duration := time.Since(start)
	d.perfMonitor.Record(method, currentEngine, duration, err)

	// Check if we should switch engines
	if d.perfMonitor.HasEnoughData(method) {
		_ = d.MaybeSwitchEngine()
	}

	return result, err
}

// MaybeSwitchEngine evaluates performance and switches engines if needed
func (d *DualEngine) MaybeSwitchEngine() error {
	perfData := d.perfMonitor.GetAveragePerformance()

	recommendedEngine := d.selector.SelectEngine(perfData)

	d.mu.RLock()
	currentEngine := d.currentType
	d.mu.RUnlock()

	if recommendedEngine != currentEngine {
		return d.SwitchEngine(recommendedEngine)
	}

	return nil
}

// SwitchEngine switches to the specified engine
func (d *DualEngine) SwitchEngine(engineType EngineType) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.currentType == engineType {
		return nil
	}

	d.currentType = engineType
	return nil
}

// Cleanup cleans up both engines
func (d *DualEngine) Cleanup() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_ = d.jsEngine.Cleanup()
	_ = d.wasmEngine.Cleanup()
	return nil
}

// CurrentEngine returns the current engine type
func (d *DualEngine) CurrentEngine() EngineType {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.currentType
}

// Stats returns combined statistics from both engines
func (d *DualEngine) Stats() *EngineStats {
	d.mu.RLock()
	currentEngine := d.currentType
	d.mu.RUnlock()

	switch currentEngine {
	case EngineJS:
		return d.jsEngine.Stats()
	case EngineWASM:
		return d.wasmEngine.Stats()
	default:
		return &EngineStats{EngineType: currentEngine}
	}
}

// GetJSEngine returns the JavaScript engine
func (d *DualEngine) GetJSEngine() *JSEngine {
	return d.jsEngine
}

// GetWASMEngine returns the WebAssembly engine
func (d *DualEngine) GetWASMEngine() *WASMEngine {
	return d.wasmEngine
}

// GetPerformanceMonitor returns the performance monitor
func (d *DualEngine) GetPerformanceMonitor() *PerformanceMonitor {
	return d.perfMonitor
}

// GetEngineSelector returns the engine selector
func (d *DualEngine) GetEngineSelector() *EngineSelector {
	return d.selector
}

// ForceEngine forces the use of a specific engine
func (d *DualEngine) ForceEngine(engineType EngineType) error {
	return d.SwitchEngine(engineType)
}

// GetGameID returns the game ID
func (d *DualEngine) GetGameID() string {
	return d.gameID
}

// PerformanceMonitor tracks engine performance
type PerformanceMonitor struct {
	records    map[string][]*PerformanceRecord
	mu         sync.RWMutex
	windowSize int
}

// PerformanceRecord represents a single performance record
type PerformanceRecord struct {
	Method      string
	EngineType  EngineType
	Duration    time.Duration
	Error       error
	Timestamp   time.Time
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		records:    make(map[string][]*PerformanceRecord),
		windowSize: 100, // Keep last 100 records per method
	}
}

// Record records a performance data point
func (p *PerformanceMonitor) Record(method string, engineType EngineType, duration time.Duration, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.records[method] == nil {
		p.records[method] = make([]*PerformanceRecord, 0, p.windowSize)
	}

	record := &PerformanceRecord{
		Method:     method,
		EngineType: engineType,
		Duration:   duration,
		Error:      err,
		Timestamp:  time.Now(),
	}

	p.records[method] = append(p.records[method], record)

	// Keep only the last N records
	if len(p.records[method]) > p.windowSize {
		p.records[method] = p.records[method][1:]
	}
}

// HasEnoughData checks if we have enough data to make a decision
func (p *PerformanceMonitor) HasEnoughData(method string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	records, ok := p.records[method]
	if !ok {
		return false
	}

	return len(records) >= 10 // Need at least 10 samples
}

// GetAveragePerformance returns average performance data
func (p *PerformanceMonitor) GetAveragePerformance() *PerformanceData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	allRecords := make([]*PerformanceRecord, 0)
	for _, records := range p.records {
		allRecords = append(allRecords, records...)
	}

	if len(allRecords) == 0 {
		return &PerformanceData{}
	}

	var totalDuration time.Duration
	var errorCount int64
	var successCount int64

	for _, record := range allRecords {
		totalDuration += record.Duration
		if record.Error != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	avgDuration := totalDuration / time.Duration(len(allRecords))

	// Calculate FPS (assuming 60 FPS target)
	avgFPS := 1000.0 / float64(avgDuration.Milliseconds())
	if avgFPS > 60 {
		avgFPS = 60
	}

	return &PerformanceData{
		AverageFPS:  avgFPS,
		ExecuteTime: avgDuration,
		ErrorCount:  errorCount,
		Complexity:  int(len(allRecords)), // Simple complexity metric
	}
}

// GetMethodPerformance returns performance data for a specific method
func (p *PerformanceMonitor) GetMethodPerformance(method string) *PerformanceData {
	p.mu.RLock()
	defer p.mu.RUnlock()

	records, ok := p.records[method]
	if !ok || len(records) == 0 {
		return &PerformanceData{}
	}

	var totalDuration time.Duration
	var errorCount int64
	var successCount int64

	for _, record := range records {
		totalDuration += record.Duration
		if record.Error != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	avgDuration := totalDuration / time.Duration(len(records))

	return &PerformanceData{
		AverageFPS:  1000.0 / float64(avgDuration.Milliseconds()),
		ExecuteTime: avgDuration,
		ErrorCount:  errorCount,
		Complexity:  len(records),
	}
}

// Clear clears all performance records
func (p *PerformanceMonitor) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.records = make(map[string][]*PerformanceRecord)
}

// ClearMethod clears records for a specific method
func (p *PerformanceMonitor) ClearMethod(method string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.records, method)
}
