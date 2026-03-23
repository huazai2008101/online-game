package engine

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/bytecodealliance/wasmtime-go"
)

// WASMEngine implements a WebAssembly game engine using wasmtime
type WASMEngine struct {
	engine   *wasmtime.Engine
	store    *wasmtime.Store
	linker   *wasmtime.Linker
	module   *wasmtime.Module
	instance *wasmtime.Instance
	cache    sync.Map
	stats    EngineStats
	gameID   string
	mu       sync.RWMutex
}

// NewWASMEngine creates a new WebAssembly engine
func NewWASMEngine() (*WASMEngine, error) {
	engine := wasmtime.NewEngine()
	store := wasmtime.NewStore(engine)
	linker := wasmtime.NewLinker(engine)

	return &WASMEngine{
		engine: engine,
		store:  store,
		linker: linker,
		stats: EngineStats{
			EngineType: EngineWASM,
		},
	}, nil
}

// Init initializes the engine with game ID and initial state
func (e *WASMEngine) Init(ctx context.Context, gameID string, state any) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.gameID = gameID
	return nil
}

// LoadScript loads a WebAssembly module
func (e *WASMEngine) LoadScript(scriptType EngineType, script []byte) error {
	if scriptType != EngineWASM {
		return fmt.Errorf("invalid script type for WASM engine: %v", scriptType)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Check cache
	key := sha256.Sum256(script)
	if cached, ok := e.cache.Load(key); ok {
		if module, ok := cached.(*wasmtime.Module); ok {
			e.module = module
			return nil
		}
	}

	// Compile WASM module
	module, err := wasmtime.NewModule(e.engine, script)
	if err != nil {
		e.stats.ErrorCount++
		return fmt.Errorf("failed to compile WASM module: %w", err)
	}

	// Cache the compiled module
	e.module = module
	e.cache.Store(key, module)

	return nil
}

// Execute executes a WebAssembly function
func (e *WASMEngine) Execute(ctx context.Context, method string, args any) (any, error) {
	start := time.Now()
	defer func() {
		e.stats.LastExecuteTime = time.Now()
	}()

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.module == nil {
		e.stats.ErrorCount++
		return nil, fmt.Errorf("no WASM module loaded")
	}

	// Create instance
	inst, err := e.linker.Instantiate(e.store, e.module)
	if err != nil {
		e.stats.ErrorCount++
		return nil, fmt.Errorf("failed to create WASM instance: %w", err)
	}
	e.instance = inst

	// Get the exported function
	funcValue := inst.GetFunc(e.store, method)
	if funcValue == nil {
		e.stats.ErrorCount++
		return nil, fmt.Errorf("function %s not found", method)
	}

	// Convert args to interface{} for wasmtime
	var goArgs []interface{}
	switch v := args.(type) {
	case int:
		goArgs = []interface{}{int32(v)}
	case int32:
		goArgs = []interface{}{v}
	case int64:
		goArgs = []interface{}{v}
	case float32:
		goArgs = []interface{}{v}
	case float64:
		goArgs = []interface{}{v}
	case []int:
		goArgs = make([]interface{}, len(v))
		for i, val := range v {
			goArgs[i] = int32(val)
		}
	case nil:
		goArgs = nil
	default:
		goArgs = nil
	}

	// Call the function
	result, err := funcValue.Call(e.store, goArgs...)

	e.stats.ExecuteCount++
	e.stats.ExecuteTime += time.Since(start).Nanoseconds()

	if err != nil {
		e.stats.ErrorCount++
		return nil, fmt.Errorf("WASM execution failed: %w", err)
	}

	// Convert result
	if result == nil {
		return nil, nil
	}

	// Try to convert result to int64
	switch v := result.(type) {
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		return result, nil
	}
}

// SwitchEngine is a no-op for the WASM engine (used by DualEngine)
func (e *WASMEngine) SwitchEngine(engineType EngineType) error {
	return fmt.Errorf("cannot switch engine on a single WASM engine")
}

// Cleanup cleans up engine resources
func (e *WASMEngine) Cleanup() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear cache
	e.cache.Range(func(key, value any) bool {
		e.cache.Delete(key)
		return true
	})

	e.module = nil
	e.instance = nil
	return nil
}

// CurrentEngine returns the current engine type
func (e *WASMEngine) CurrentEngine() EngineType {
	return EngineWASM
}

// Stats returns the engine statistics
func (e *WASMEngine) Stats() *EngineStats {
	return &e.stats
}

// ClearCache clears the compiled module cache
func (e *WASMEngine) ClearCache() {
	e.cache.Range(func(key, value any) bool {
		e.cache.Delete(key)
		return true
	})
}
