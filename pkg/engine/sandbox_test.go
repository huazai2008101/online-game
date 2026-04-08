package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSandboxConfig_Values(t *testing.T) {
	cfg := DefaultSandboxConfig()
	assert.Equal(t, int64(5000), cfg.ExecutionTimeoutMs)
	assert.Equal(t, int64(512*1024), cfg.MaxScriptSize)
	assert.Equal(t, 100, cfg.MaxCallStackSize)
	assert.Equal(t, 50, cfg.MaxActiveTimers)
}

func TestSandboxConfig_ZeroValues(t *testing.T) {
	var cfg SandboxConfig
	assert.Equal(t, int64(0), cfg.ExecutionTimeoutMs)
	assert.Equal(t, int64(0), cfg.MaxScriptSize)
	assert.Equal(t, 0, cfg.MaxCallStackSize)
	assert.Equal(t, 0, cfg.MaxActiveTimers)
}

func TestEngineConfig_SandboxField(t *testing.T) {
	cfg := EngineConfig{
		GameID: "test",
		RoomID: "room1",
	}
	assert.Equal(t, SandboxConfig{}, cfg.Sandbox)

	cfg.Sandbox = DefaultSandboxConfig()
	assert.Equal(t, int64(5000), cfg.Sandbox.ExecutionTimeoutMs)
}
