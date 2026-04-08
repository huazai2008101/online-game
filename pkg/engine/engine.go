package engine

import (
	"context"
)

// GameEngine defines the interface for executing game scripts.
// Each room gets its own engine instance for full isolation.
type GameEngine interface {
	// Init initializes the engine with game context and loads the game script.
	Init(ctx context.Context, config *EngineConfig) error

	// TriggerHook invokes a lifecycle or event hook in the game script.
	// Returns the hook's return value and any execution error.
	TriggerHook(hookName string, args ...any) (any, error)

	// LoadScript loads a game script into the engine.
	LoadScript(scriptContent string) error

	// GetState exports the current game state from the JS sandbox.
	GetState() map[string]any

	// Destroy terminates the engine and releases all resources.
	Destroy()
}

// EngineConfig holds configuration for initializing an engine instance.
type EngineConfig struct {
	GameID     string
	RoomID     string
	GameCode   string
	Version    string
	ScriptPath string
	MaxMemory  int64        // deprecated: use Sandbox fields
	Timeout    int64        // deprecated: use Sandbox.ExecutionTimeoutMs
	Env        map[string]any
	Sandbox    SandboxConfig // resource limits for the JS sandbox
}

// EngineStats holds runtime statistics.
type EngineStats struct {
	ExecuteCount    int64  `json:"execute_count"`
	ErrorCount      int64  `json:"error_count"`
	TotalExecTimeMs int64  `json:"total_exec_time_ms"`
	MemoryUsage     int64  `json:"memory_usage_bytes"`
	UptimeMs        int64  `json:"uptime_ms"`
}
