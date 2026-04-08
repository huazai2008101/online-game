package engine

// SandboxConfig holds resource limits for the goja JS execution sandbox.
// All limits are per-engine-instance (per-game-room).
type SandboxConfig struct {
	// ExecutionTimeoutMs is the max wall-clock time a single hook may run.
	// 0 = no timeout. Recommended: 5000ms.
	ExecutionTimeoutMs int64

	// MaxScriptSize is the max allowed game script size in bytes.
	// 0 = no limit. Recommended: 512KB.
	MaxScriptSize int64

	// MaxCallStackSize limits the JS call stack depth.
	// 0 = goja default. Recommended: 100.
	MaxCallStackSize int

	// MaxActiveTimers limits simultaneous setTimeout + setInterval timers.
	// 0 = no limit. Recommended: 50.
	MaxActiveTimers int
}

// DefaultSandboxConfig returns production-ready defaults.
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		ExecutionTimeoutMs: 5000,            // 5 seconds per hook
		MaxScriptSize:      512 * 1024,      // 512 KB
		MaxCallStackSize:   100,
		MaxActiveTimers:    50,
	}
}
