package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock adapters ---

type testHub struct {
	mu         sync.Mutex
	broadcasts []string // events recorded
}

func (h *testHub) BroadcastToRoom(_ string, event string, _ any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.broadcasts = append(h.broadcasts, event)
}

func (h *testHub) SendTo(_ string, _ string, _ any)   {}
func (h *testHub) SendExcept(_ string, _ string, _ string, _ any) {}

func (h *testHub) getEvents() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string{}, h.broadcasts...)
}

type testRoomMgr struct{}

func (t *testRoomMgr) GetPlayerIDs(_ string) []string        { return []string{"p1", "p2"} }
func (t *testRoomMgr) GetConfig(_ string) map[string]any     { return nil }
func (t *testRoomMgr) RecordGameResults(_ string, _ any)     {}

// --- Helpers ---

func newTestEngine() *GojaEngine {
	return NewGojaEngine(&testHub{}, &testRoomMgr{})
}

func initEngine(t *testing.T, sandbox SandboxConfig) *GojaEngine {
	t.Helper()
	eng := newTestEngine()
	cfg := &EngineConfig{
		GameID:   "g1",
		RoomID:   "r1",
		GameCode: "test",
		Version:  "1.0",
		Sandbox:  sandbox,
	}
	require.NoError(t, eng.Init(context.Background(), cfg))
	return eng
}

// --- Tests ---

func TestGojaEngine_InitAndLoadScript(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	defer eng.Destroy()

	script := `
		GameServer.onInit = function(ctx) {
			GameServer._state.initialized = true;
		};
	`
	require.NoError(t, eng.LoadScript(script))

	// Trigger onInit
	_, err := eng.TriggerHook("onInit", map[string]any{"gameId": "g1"})
	require.NoError(t, err)

	state := eng.GetState()
	assert.NotNil(t, state)
}

func TestGojaEngine_TriggerHook_ReturnValue(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	defer eng.Destroy()

	script := `
		var onCustomHook = function(a, b) {
			return a + b;
		};
	`
	require.NoError(t, eng.LoadScript(script))

	result, err := eng.TriggerHook("onCustomHook", 3, 4)
	require.NoError(t, err)
	assert.Equal(t, int64(7), result)
}

func TestGojaEngine_TriggerHook_UndefinedHook(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	defer eng.Destroy()

	require.NoError(t, eng.LoadScript(""))

	// Hook not defined by developer — should return nil
	result, err := eng.TriggerHook("onPlayerAction", "p1", "bet", 100)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestGojaEngine_TriggerHook_NotInitialized(t *testing.T) {
	eng := newTestEngine()
	_, err := eng.TriggerHook("onInit")
	assert.Error(t, err)
}

func TestGojaEngine_Sandbox_EvalDisabled(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	defer eng.Destroy()

	script := `
		try {
			eval("1+1");
			var evalWorked = true;
		} catch(e) {
			var evalWorked = false;
		}
	`
	require.NoError(t, eng.LoadScript(script))

	// Verify eval throws by loading a script that uses eval
	eng2 := initEngine(t, SandboxConfig{})
	defer eng2.Destroy()

	err := eng2.LoadScript(`eval("1+1");`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "eval is disabled")
}

func TestGojaEngine_Sandbox_ScriptSizeLimit(t *testing.T) {
	eng := initEngine(t, SandboxConfig{
		MaxScriptSize:      100, // very small limit
		ExecutionTimeoutMs: 5000,
		MaxCallStackSize:   100,
		MaxActiveTimers:    50,
	})
	defer eng.Destroy()

	// Script larger than 100 bytes (this string is ~200 bytes)
	largeScript := "var x = 1; var y = 2; var z = 3; var w = 4; var v = 5; var a = 6; var b = 7; var c = 8; var d = 9; var e = 10; var f = 11; var g = 12; var h = 13; var i = 14; var j = 15; // padding to exceed limit"
	err := eng.LoadScript(largeScript)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds max size")
}

func TestGojaEngine_Sandbox_ExecutionTimeout(t *testing.T) {
	eng := initEngine(t, SandboxConfig{
		ExecutionTimeoutMs: 100, // 100ms timeout
		MaxCallStackSize:   100,
		MaxActiveTimers:    50,
	})
	defer eng.Destroy()

	script := `
		var onInfiniteLoop = function() {
			var start = Date.now();
			while (true) {}
		};
	`
	require.NoError(t, eng.LoadScript(script))

	_, err := eng.TriggerHook("onInfiniteLoop")
	assert.Error(t, err)
}

func TestGojaEngine_Sandbox_CallStackLimit(t *testing.T) {
	eng := initEngine(t, SandboxConfig{
		MaxCallStackSize:   10,
		ExecutionTimeoutMs: 5000,
		MaxActiveTimers:    50,
	})
	defer eng.Destroy()

	script := `
		var onDeepRecursion = function() {
			function recurse(n) {
				if (n <= 0) return 0;
				return recurse(n - 1);
			}
			recurse(100); // exceeds call stack of 10
		};
	`
	require.NoError(t, eng.LoadScript(script))

	_, err := eng.TriggerHook("onDeepRecursion")
	assert.Error(t, err)
}

func TestGojaEngine_Sandbox_DefaultConfig(t *testing.T) {
	// Zero-value sandbox should get defaults
	eng := newTestEngine()
	cfg := &EngineConfig{
		GameID:   "g1",
		RoomID:   "r1",
		GameCode: "test",
		Version:  "1.0",
	}
	require.NoError(t, eng.Init(context.Background(), cfg))
	defer eng.Destroy()

	assert.Equal(t, int64(5000), eng.sandbox.ExecutionTimeoutMs)
	assert.Equal(t, 100, eng.sandbox.MaxCallStackSize)
}

func TestGojaEngine_TimerCallback(t *testing.T) {
	eng := initEngine(t, SandboxConfig{
		ExecutionTimeoutMs: 5000,
		MaxCallStackSize:   100,
		MaxActiveTimers:    50,
	})
	defer eng.Destroy()

	var timerFired atomic.Int32
	eng.SetTimerCallback(func(timerID int) {
		timerFired.Add(1)
	})

	script := `
		var onTestTimer = function() {
			GameServer.setTimeout(function() {}, 50);
		};
	`
	require.NoError(t, eng.LoadScript(script))

	_, err := eng.TriggerHook("onTestTimer")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return timerFired.Load() == 1
	}, 2*time.Second, 20*time.Millisecond)
}

func TestGojaEngine_TimerLimit(t *testing.T) {
	eng := initEngine(t, SandboxConfig{
		ExecutionTimeoutMs: 5000,
		MaxCallStackSize:   100,
		MaxActiveTimers:    2, // only 2 timers allowed
	})
	defer eng.Destroy()

	var timerFired atomic.Int32
	eng.SetTimerCallback(func(timerID int) {
		timerFired.Add(1)
	})

	script := `
		var onTestTimerLimit = function() {
			GameServer.setTimeout(function() {}, 5000); // timer 1
			GameServer.setTimeout(function() {}, 5000); // timer 2
			GameServer.setTimeout(function() {}, 5000); // timer 3 — should be dropped
		};
	`
	require.NoError(t, eng.LoadScript(script))

	_, err := eng.TriggerHook("onTestTimerLimit")
	require.NoError(t, err)

	// Only 2 timers should be active
	assert.Len(t, eng.timers, 2)
}

func TestGojaEngine_HostAPI_Broadcast(t *testing.T) {
	hub := &testHub{}
	eng := NewGojaEngine(hub, &testRoomMgr{})
	cfg := &EngineConfig{
		GameID:   "g1",
		RoomID:   "r1",
		GameCode: "test",
		Sandbox:  DefaultSandboxConfig(),
	}
	require.NoError(t, eng.Init(context.Background(), cfg))
	defer eng.Destroy()

	script := `
		var onTestBroadcast = function() {
			GameServer.broadcast("test_event", { key: "value" });
		};
	`
	require.NoError(t, eng.LoadScript(script))

	_, err := eng.TriggerHook("onTestBroadcast")
	require.NoError(t, err)

	events := hub.getEvents()
	assert.Contains(t, events, "test_event")
}

func TestGojaEngine_HostAPI_RandomInt(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	defer eng.Destroy()

	script := `
		var onTestRandom = function() {
			return GameServer.randomInt(1, 10);
		};
	`
	require.NoError(t, eng.LoadScript(script))

	result, err := eng.TriggerHook("onTestRandom")
	require.NoError(t, err)

	n, ok := result.(int64)
	require.True(t, ok)
	assert.GreaterOrEqual(t, n, int64(1))
	assert.LessOrEqual(t, n, int64(10))
}

func TestGojaEngine_HostAPI_Log(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	defer eng.Destroy()

	// log should not panic
	script := `
		var onTestLog = function() {
			GameServer.log("hello", "world");
			GameServer.warn("warning");
			GameServer.error("error");
			return true;
		};
	`
	require.NoError(t, eng.LoadScript(script))

	result, err := eng.TriggerHook("onTestLog")
	require.NoError(t, err)
	assert.Equal(t, true, result)
}

func TestGojaEngine_HostAPI_Shuffle(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	defer eng.Destroy()

	script := `
		var onTestShuffle = function() {
			var arr = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];
			var shuffled = GameServer.shuffle(arr);
			return shuffled;
		};
	`
	require.NoError(t, eng.LoadScript(script))

	result, err := eng.TriggerHook("onTestShuffle")
	require.NoError(t, err)

	arr, ok := result.([]interface{})
	require.True(t, ok)
	assert.Len(t, arr, 10)
}

func TestGojaEngine_SDKRuntime_GameServerObject(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	defer eng.Destroy()

	// Empty script — SDK runtime should already provide GameServer
	require.NoError(t, eng.LoadScript(""))

	// Verify GameServer.getPlayers works
	script2 := `
		var onTestPlayers = function() {
			return GameServer.getPlayers();
		};
	`
	// Need to re-init for second script since LoadScript appends to existing runtime
	eng2 := initEngine(t, SandboxConfig{})
	defer eng2.Destroy()
	require.NoError(t, eng2.LoadScript(script2))

	result, err := eng2.TriggerHook("onTestPlayers")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGojaEngine_LifecycleHooks(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	defer eng.Destroy()

	script := `
		var callLog = [];
		GameServer.onInit = function(ctx) { callLog.push("init"); };
		GameServer.onPlayerJoin = function(pid, info) { callLog.push("join:" + pid); };
		GameServer.onGameStart = function() { callLog.push("start"); };
		GameServer.onPlayerAction = function(pid, action, data) { callLog.push("action:" + action); };

		var getCallLog = function() { return callLog; };
	`
	require.NoError(t, eng.LoadScript(script))

	eng.TriggerHook("onInit", map[string]any{"gameId": "g1"})
	eng.TriggerHook("onPlayerJoin", "p1", map[string]any{"nickname": "A"})
	eng.TriggerHook("onGameStart")
	eng.TriggerHook("onPlayerAction", "p1", "bet", 100)

	result, err := eng.TriggerHook("getCallLog")
	require.NoError(t, err)

	log, ok := result.([]interface{})
	require.True(t, ok)
	assert.Equal(t, []interface{}{"init", "join:p1", "start", "action:bet"}, log)
}

func TestGojaEngine_GetState(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	defer eng.Destroy()

	script := `
		GameServer.onInit = function(ctx) {
			GameServer.state.phase = "waiting";
			GameServer.state.players = {};
		};
	`
	require.NoError(t, eng.LoadScript(script))

	eng.TriggerHook("onInit", map[string]any{})

	state := eng.GetState()
	assert.NotNil(t, state)
}

func TestGojaEngine_Destroy(t *testing.T) {
	eng := initEngine(t, SandboxConfig{})
	eng.Destroy()
	assert.Nil(t, eng.vm)
}
