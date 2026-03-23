package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robertkrimen/otto"
)

// JSEngine implements a JavaScript game engine using otto (V8)
type JSEngine struct {
	vm            *otto.Otto
	vmPool        sync.Pool
	cache         sync.Map
	stats         EngineStats
	warmupScripts []string
	gameID        string
	mu            sync.RWMutex
}

// NewJSEngine creates a new JavaScript engine
func NewJSEngine() *JSEngine {
	return &JSEngine{
		vm: otto.New(),
		stats: EngineStats{
			EngineType: EngineJS,
		},
		vmPool: sync.Pool{
			New: func() any {
				return otto.New()
			},
		},
		warmupScripts: make([]string, 0),
	}
}

// Init initializes the engine with game ID and initial state
func (e *JSEngine) Init(ctx context.Context, gameID string, state any) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.gameID = gameID

	// Set global variables
	if err := e.vm.Set("gameID", gameID); err != nil {
		return fmt.Errorf("failed to set gameID: %w", err)
	}

	if err := e.vm.Set("state", state); err != nil {
		return fmt.Errorf("failed to set state: %w", err)
	}

	// Set up console object
	console, err := e.vm.Object("({})")
	if err != nil {
		return err
	}

	_ = console.Set("log", func(call otto.FunctionCall) otto.Value {
		fmt.Printf("[JS] %s: %v\n", gameID, call.ArgumentList)
		return otto.UndefinedValue()
	})

	_ = console.Set("error", func(call otto.FunctionCall) otto.Value {
		fmt.Printf("[JS ERROR] %s: %v\n", gameID, call.ArgumentList)
		return otto.UndefinedValue()
	})

	_ = e.vm.Set("console", console)

	// Run warmup scripts
	for _, script := range e.warmupScripts {
		if _, err := e.vm.Run(script); err != nil {
			return fmt.Errorf("warmup script failed: %w", err)
		}
	}

	return nil
}

// LoadScript loads a JavaScript script
func (e *JSEngine) LoadScript(scriptType EngineType, script []byte) error {
	if scriptType != EngineJS {
		return fmt.Errorf("invalid script type for JS engine: %v", scriptType)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, err := e.vm.Run(string(script)); err != nil {
		e.stats.ErrorCount++
		return fmt.Errorf("failed to load script: %w", err)
	}

	return nil
}

// Execute executes a JavaScript method
func (e *JSEngine) Execute(ctx context.Context, method string, args any) (any, error) {
	start := time.Now()
	defer func() {
		e.stats.LastExecuteTime = time.Now()
	}()

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%v", method, args)
	if cached, ok := e.cache.Load(cacheKey); ok {
		result, err := cached.(func(context.Context, any) (any, error))(ctx, args)
		e.stats.mu.Lock()
		e.stats.CacheHitCount++
		e.stats.ExecuteCount++
		e.stats.ExecuteTime += time.Since(start).Nanoseconds()
		e.stats.mu.Unlock()
		if err != nil {
			e.stats.mu.Lock()
			e.stats.ErrorCount++
			e.stats.mu.Unlock()
			return nil, err
		}
		return result, nil
	}

	e.mu.RLock()
	vm := e.vm
	e.mu.RUnlock()

	// Execute the method
	value, err := vm.Call(method, nil, args)

	e.stats.mu.Lock()
	e.stats.CacheMissCount++
	e.stats.ExecuteCount++
	e.stats.ExecuteTime += time.Since(start).Nanoseconds()
	e.stats.mu.Unlock()

	if err != nil {
		e.stats.mu.Lock()
		e.stats.ErrorCount++
		e.stats.mu.Unlock()
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	// Export and cache result
	result, err := value.Export()
	if err != nil {
		e.stats.mu.Lock()
		e.stats.ErrorCount++
		e.stats.mu.Unlock()
		return nil, fmt.Errorf("export failed: %w", err)
	}

	// Cache the result if it's cacheable
	e.cache.Store(cacheKey, func(ctx context.Context, args any) (any, error) {
		return result, nil
	})

	return result, nil
}

// SwitchEngine is a no-op for the JS engine (used by DualEngine)
func (e *JSEngine) SwitchEngine(engineType EngineType) error {
	return fmt.Errorf("cannot switch engine on a single JS engine")
}

// Cleanup cleans up engine resources
func (e *JSEngine) Cleanup() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear cache
	e.cache.Range(func(key, value any) bool {
		e.cache.Delete(key)
		return true
	})

	// Create new VM to clear state
	e.vm = otto.New()
	return nil
}

// CurrentEngine returns the current engine type
func (e *JSEngine) CurrentEngine() EngineType {
	return EngineJS
}

// Stats returns the engine statistics
func (e *JSEngine) Stats() *EngineStats {
	e.stats.mu.RLock()
	defer e.stats.mu.RUnlock()
	return &e.stats
}

// AddWarmupScript adds a script to run during initialization
func (e *JSEngine) AddWarmupScript(script string) {
	e.warmupScripts = append(e.warmupScripts, script)
}

// ClearCache clears the execution cache
func (e *JSEngine) ClearCache() {
	e.cache.Range(func(key, value any) bool {
		e.cache.Delete(key)
		return true
	})
	e.stats.mu.Lock()
	e.stats.CacheHitCount = 0
	e.stats.CacheMissCount = 0
	e.stats.mu.Unlock()
}

// RunCode runs arbitrary JavaScript code
func (e *JSEngine) RunCode(code string) (any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	value, err := e.vm.Run(code)
	if err != nil {
		e.stats.mu.Lock()
		e.stats.ErrorCount++
		e.stats.mu.Unlock()
		return nil, err
	}

	return value.Export()
}

// SetGlobal sets a global variable in the JavaScript VM
func (e *JSEngine) SetGlobal(name string, value any) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.vm.Set(name, value)
}

// GetGlobal gets a global variable from the JavaScript VM
func (e *JSEngine) GetGlobal(name string) (any, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	value, err := e.vm.Get(name)
	if err != nil {
		return nil, err
	}

	return value.Export()
}
