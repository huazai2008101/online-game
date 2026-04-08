package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

// GojaEngine implements GameEngine using the goja JavaScript runtime.
// IMPORTANT: Each GojaEngine instance MUST only be accessed from a single goroutine
// (the owning GameActor's goroutine). This is enforced by the Actor model.
type GojaEngine struct {
	vm       *goja.Runtime
	config   *EngineConfig
	hub      HubAdapter
	roomMgr  RoomAdapter
	sandbox  SandboxConfig

	// Timer management (accessed only from actor goroutine)
	timers  map[int]*time.Timer
	timerID atomic.Int32

	// State
	state     map[string]any
	startTime time.Time
	stats     EngineStats

	// Callbacks
	onGameEnd       func(roomID string, results any)
	onTimerCallback func(timerID int) // called from time.AfterFunc to route back to actor inbox

	mu sync.Mutex // only used for external GetState() calls
}

// HubAdapter abstracts WebSocket message delivery for the engine.
type HubAdapter interface {
	BroadcastToRoom(roomID string, event string, data any)
	SendTo(playerID string, event string, data any)
	SendExcept(roomID string, exceptPlayerID string, event string, data any)
}

// RoomAdapter abstracts room management for the engine.
type RoomAdapter interface {
	GetPlayerIDs(roomID string) []string
	GetConfig(roomID string) map[string]any
	RecordGameResults(roomID string, results any)
}

// NewGojaEngine creates a new goja-based game engine.
func NewGojaEngine(hub HubAdapter, roomMgr RoomAdapter) *GojaEngine {
	return &GojaEngine{
		hub:     hub,
		roomMgr: roomMgr,
		timers:  make(map[int]*time.Timer),
		state:   make(map[string]any),
	}
}

// Init initializes the goja runtime and injects host APIs.
func (e *GojaEngine) Init(ctx context.Context, config *EngineConfig) error {
	e.config = config
	e.vm = goja.New()
	e.startTime = time.Now()

	// Apply sandbox config (use defaults if zero-valued)
	e.sandbox = config.Sandbox
	if e.sandbox.ExecutionTimeoutMs == 0 && e.sandbox.MaxCallStackSize == 0 {
		e.sandbox = DefaultSandboxConfig()
		config.Sandbox = e.sandbox
	}

	// Apply call stack limit
	if e.sandbox.MaxCallStackSize > 0 {
		e.vm.SetMaxCallStackSize(e.sandbox.MaxCallStackSize)
	}

	// Disable eval for security
	vm := e.vm
	vm.Set("eval", func(call goja.FunctionCall) goja.Value {
		panic(vm.NewGoError(fmt.Errorf("eval is disabled in sandbox")))
	})

	// Inject all host APIs before loading game script
	e.injectHostAPI()

	slog.Info("goja engine initialized",
		"game_id", config.GameID,
		"room_id", config.RoomID,
		"timeout_ms", e.sandbox.ExecutionTimeoutMs,
		"max_timers", e.sandbox.MaxActiveTimers,
	)
	return nil
}

// LoadScript loads and executes the game script in the goja runtime.
func (e *GojaEngine) LoadScript(scriptContent string) error {
	if e.vm == nil {
		return fmt.Errorf("engine not initialized")
	}

	// Enforce script size limit
	if e.sandbox.MaxScriptSize > 0 && int64(len(scriptContent)) > e.sandbox.MaxScriptSize {
		return fmt.Errorf("script exceeds max size (%d > %d bytes)",
			len(scriptContent), e.sandbox.MaxScriptSize)
	}

	// Load the SDK runtime first (provides GameServer base class)
	sdkRuntime := e.getSDKRuntime()
	if _, err := e.vm.RunString(sdkRuntime); err != nil {
		return fmt.Errorf("load SDK runtime: %w", err)
	}

	// Load the game script
	if _, err := e.vm.RunString(scriptContent); err != nil {
		return fmt.Errorf("load game script: %w", err)
	}

	slog.Info("game script loaded",
		"game_id", e.config.GameID,
		"room_id", e.config.RoomID,
	)
	return nil
}

// TriggerHook invokes a named hook function in the JS game script.
// Enforces execution timeout via vm.Interrupt().
func (e *GojaEngine) TriggerHook(hookName string, args ...any) (any, error) {
	if e.vm == nil {
		return nil, fmt.Errorf("engine not initialized")
	}

	start := time.Now()
	defer func() {
		e.stats.ExecuteCount++
		e.stats.TotalExecTimeMs += time.Since(start).Milliseconds()
	}()

	// Panic recovery for JS execution errors
	defer func() {
		if r := recover(); r != nil {
			e.stats.ErrorCount++
			slog.Error("goja execution panic",
				"hook", hookName,
				"panic", r,
			)
		}
	}()

	fn, ok := goja.AssertFunction(e.vm.Get(hookName))
	if !ok {
		return nil, nil // hook not defined, optional
	}

	// Setup execution timeout
	var timeoutTimer *time.Timer
	timeoutMs := e.sandbox.ExecutionTimeoutMs
	if timeoutMs > 0 {
		timeoutTimer = time.AfterFunc(time.Duration(timeoutMs)*time.Millisecond, func() {
			e.vm.Interrupt("execution timeout exceeded")
		})
		defer func() {
			if timeoutTimer != nil {
				timeoutTimer.Stop()
			}
		}()
	}

	goArgs := make([]goja.Value, len(args))
	for i, arg := range args {
		goArgs[i] = e.vm.ToValue(arg)
	}

	result, err := fn(nil, goArgs...)
	if err != nil {
		e.stats.ErrorCount++
		// Distinguish timeout from other errors
		if timeoutTimer != nil && !timeoutTimer.Stop() {
			// Timer already fired — this was a timeout
			return nil, fmt.Errorf("hook %s timed out after %dms", hookName, timeoutMs)
		}
		return nil, fmt.Errorf("hook %s execution error: %w", hookName, err)
	}

	if result != nil && result != goja.Undefined() && result != goja.Null() {
		return result.Export(), nil
	}
	return nil, nil
}

// GetState returns the current game state.
func (e *GojaEngine) GetState() map[string]any {
	return e.state
}

// Destroy cleans up all resources.
func (e *GojaEngine) Destroy() {
	// Clear all timers
	for id, t := range e.timers {
		t.Stop()
		delete(e.timers, id)
	}

	// Clear VM
	e.vm = nil

	slog.Info("goja engine destroyed",
		"game_id", e.config.GameID,
		"room_id", e.config.RoomID,
		"exec_count", e.stats.ExecuteCount,
		"errors", e.stats.ErrorCount,
	)
}

// injectHostAPI injects platform functions into the goja sandbox.
// These __platform_* functions are called by the SDK runtime (GameServer base class).
func (e *GojaEngine) injectHostAPI() {
	vm := e.vm
	roomID := e.config.RoomID

	// ---- Communication ----

	vm.Set("__platform_broadcast", func(call goja.FunctionCall) goja.Value {
		event := call.Argument(0).String()
		data := e.safeExport(call.Argument(1))
		e.hub.BroadcastToRoom(roomID, event, data)
		return vm.ToValue(true)
	})

	vm.Set("__platform_sendTo", func(call goja.FunctionCall) goja.Value {
		playerID := call.Argument(0).String()
		event := call.Argument(1).String()
		data := e.safeExport(call.Argument(2))
		e.hub.SendTo(playerID, event, data)
		return vm.ToValue(true)
	})

	vm.Set("__platform_sendExcept", func(call goja.FunctionCall) goja.Value {
		exceptID := call.Argument(0).String()
		event := call.Argument(1).String()
		data := e.safeExport(call.Argument(2))
		e.hub.SendExcept(roomID, exceptID, event, data)
		return vm.ToValue(true)
	})

	// ---- Game Control ----

	vm.Set("__platform_endGame", func(call goja.FunctionCall) goja.Value {
		results := e.safeExport(call.Argument(0))
		if e.onGameEnd != nil {
			e.onGameEnd(roomID, results)
		}
		e.hub.BroadcastToRoom(roomID, "game_end", results)
		return vm.ToValue(true)
	})

	// ---- Room Info ----

	vm.Set("__platform_getPlayers", func(call goja.FunctionCall) goja.Value {
		ids := e.roomMgr.GetPlayerIDs(roomID)
		return vm.ToValue(ids)
	})

	vm.Set("__platform_getRoomConfig", func(call goja.FunctionCall) goja.Value {
		cfg := e.roomMgr.GetConfig(roomID)
		return vm.ToValue(cfg)
	})

	vm.Set("__platform_getRoomId", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(roomID)
	})

	vm.Set("__platform_getGameId", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(e.config.GameID)
	})

	// ---- Timers ----

	vm.Set("__platform_setTimeout", func(call goja.FunctionCall) goja.Value {
		if e.sandbox.MaxActiveTimers > 0 && len(e.timers) >= e.sandbox.MaxActiveTimers {
			slog.Warn("timer limit reached", "room_id", roomID, "count", len(e.timers))
			return goja.Undefined()
		}
		ms := call.Argument(1).ToInteger()
		id := int(e.timerID.Add(1))
		e.timers[id] = time.AfterFunc(time.Duration(ms)*time.Millisecond, func() {
			if e.onTimerCallback != nil {
				e.onTimerCallback(id)
			}
		})
		return vm.ToValue(id)
	})

	vm.Set("__platform_setInterval", func(call goja.FunctionCall) goja.Value {
		if e.sandbox.MaxActiveTimers > 0 && len(e.timers) >= e.sandbox.MaxActiveTimers {
			slog.Warn("timer limit reached", "room_id", roomID, "count", len(e.timers))
			return goja.Undefined()
		}
		ms := call.Argument(1).ToInteger()
		id := int(e.timerID.Add(1))
		ticker := time.NewTicker(time.Duration(ms) * time.Millisecond)
		e.timers[id] = &time.Timer{} // placeholder for cleanup tracking
		go func() {
			for range ticker.C {
				if e.onTimerCallback != nil {
					e.onTimerCallback(id)
				} else {
					ticker.Stop()
					return
				}
			}
		}()
		return vm.ToValue(id)
	})

	vm.Set("__platform_clearTimeout", func(call goja.FunctionCall) goja.Value {
		id := int(call.Argument(0).ToInteger())
		if t, ok := e.timers[id]; ok {
			t.Stop()
			delete(e.timers, id)
		}
		return goja.Undefined()
	})

	vm.Set("__platform_clearInterval", func(call goja.FunctionCall) goja.Value {
		id := int(call.Argument(0).ToInteger())
		if t, ok := e.timers[id]; ok {
			t.Stop()
			delete(e.timers, id)
		}
		return goja.Undefined()
	})

	// ---- Utilities ----

	vm.Set("__platform_randomInt", func(call goja.FunctionCall) goja.Value {
		min := int(call.Argument(0).ToInteger())
		max := int(call.Argument(1).ToInteger())
		if max <= min {
			return vm.ToValue(min)
		}
		n := min + int(time.Now().UnixNano())%(max-min+1)
		return vm.ToValue(n)
	})

	vm.Set("__platform_shuffle", func(call goja.FunctionCall) goja.Value {
		arr, ok := call.Argument(0).Export().([]any)
		if !ok {
			return call.Argument(0)
		}
		shuffled := make([]any, len(arr))
		copy(shuffled, arr)
		for i := len(shuffled) - 1; i > 0; i-- {
			j := int(time.Now().UnixNano()) % (i + 1)
			if j < 0 {
				j = -j
			}
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		}
		return vm.ToValue(shuffled)
	})

	vm.Set("__platform_randomChoice", func(call goja.FunctionCall) goja.Value {
		arr, ok := call.Argument(0).Export().([]any)
		if !ok || len(arr) == 0 {
			return goja.Undefined()
		}
		idx := int(time.Now().UnixNano()) % len(arr)
		if idx < 0 {
			idx = -idx
		}
		return vm.ToValue(arr[idx])
	})

	vm.Set("__platform_uuid", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(fmt.Sprintf("%d-%d", time.Now().UnixNano(), e.timerID.Add(1)))
	})

	// ---- Console ----

	vm.Set("__platform_log", func(call goja.FunctionCall) goja.Value {
		args := make([]any, len(call.Arguments))
		for i, a := range call.Arguments {
			args[i] = a.Export()
		}
		slog.Info("[JS]", "args", args)
		return goja.Undefined()
	})

	vm.Set("__platform_warn", func(call goja.FunctionCall) goja.Value {
		args := make([]any, len(call.Arguments))
		for i, a := range call.Arguments {
			args[i] = a.Export()
		}
		slog.Warn("[JS]", "args", args)
		return goja.Undefined()
	})

	vm.Set("__platform_error", func(call goja.FunctionCall) goja.Value {
		args := make([]any, len(call.Arguments))
		for i, a := range call.Arguments {
			args[i] = a.Export()
		}
		slog.Error("[JS]", "args", args)
		return goja.Undefined()
	})

	// ---- State ----

	vm.Set("__platform_getState", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(e.state)
	})

	vm.Set("__platform_setState", func(call goja.FunctionCall) goja.Value {
		e.state = e.safeExport(call.Argument(0)).(map[string]any)
		return vm.ToValue(true)
	})
}

// getSDKRuntime returns the JavaScript SDK runtime code.
// This provides the GameServer base object and global lifecycle hooks.
//
// Architecture:
//   1. GameServer object provides platform APIs (broadcast, sendTo, timers, etc.)
//   2. Game developers assign lifecycle hooks to GameServer (GameServer.onPlayerJoin = fn)
//   3. Global hook functions delegate to GameServer methods
//   4. The Go engine calls vm.Get("onPlayerJoin") -> finds the global -> calls GameServer.onPlayerJoin
//
// This allows both styles:
//   - Class-based: import SDK -> class extends GameServer -> register(MyClass)
//   - Direct: GameServer.onInit = function(ctx) { ... }
func (e *GojaEngine) getSDKRuntime() string {
	return `
// ===== Game Platform SDK Runtime (injected by platform) =====
// This provides the GameServer object and global lifecycle hooks.

var GameServer = {
	_state: {},
	_playerData: {},

	// State management
	get state() { return this._state; },
	set state(v) { this._state = v; __platform_setState(v); },

	getPublicState: function() { return this._state; },

	// Communication
	broadcast: function(event, data) { __platform_broadcast(event, data); },
	sendTo: function(playerId, event, data) { __platform_sendTo(playerId, event, data); },
	sendExcept: function(playerId, event, data) { __platform_sendExcept(playerId, event, data); },

	// Game control
	endGame: function(results) { __platform_endGame(results); },

	// Room info
	getPlayers: function() { return __platform_getPlayers(); },
	getPlayerCount: function() { var p = __platform_getPlayers(); return p ? p.length : 0; },
	getRoomConfig: function() { return __platform_getRoomConfig(); },
	getRoomId: function() { return __platform_getRoomId(); },
	getGameId: function() { return __platform_getGameId(); },

	// Player data
	getPlayerData: function(playerId) {
		if (!this._playerData[playerId]) this._playerData[playerId] = {};
		return this._playerData[playerId];
	},
	setPlayerData: function(playerId, key, value) {
		if (!this._playerData[playerId]) this._playerData[playerId] = {};
		this._playerData[playerId][key] = value;
	},

	// Timers
	setTimeout: function(fn, ms) { return __platform_setTimeout(fn, ms); },
	setInterval: function(fn, ms) { return __platform_setInterval(fn, ms); },
	clearTimeout: function(id) { __platform_clearTimeout(id); },
	clearInterval: function(id) { __platform_clearInterval(id); },

	// Utilities
	randomInt: function(min, max) { return __platform_randomInt(min, max); },
	shuffle: function(arr) { return __platform_shuffle(arr); },
	randomChoice: function(arr) { return __platform_randomChoice(arr); },
	uuid: function() { return __platform_uuid(); },

	// Logging
	log: function() {
		var args = Array.prototype.slice.call(arguments);
		__platform_log.apply(null, args);
	},
	warn: function() {
		var args = Array.prototype.slice.call(arguments);
		__platform_warn.apply(null, args);
	},
	error: function() {
		var args = Array.prototype.slice.call(arguments);
		__platform_error.apply(null, args);
	}
};

// Convenient aliases
var game = GameServer;
var room = GameServer;
var player = GameServer;
var timer = GameServer;
var random = GameServer;

// ===== Global Lifecycle Hooks =====
// The Go engine calls vm.Get("onPlayerJoin") etc.
// These global functions delegate to GameServer methods.
// Game developers assign: GameServer.onPlayerJoin = function(pid, info) { ... }

var onInit = function(ctx) {
	if (typeof GameServer.onInit === 'function') GameServer.onInit(ctx);
};
var onPlayerJoin = function(playerId, info) {
	if (typeof GameServer.onPlayerJoin === 'function') GameServer.onPlayerJoin(playerId, info);
};
var onPlayerLeave = function(playerId, reason) {
	if (typeof GameServer.onPlayerLeave === 'function') GameServer.onPlayerLeave(playerId, reason);
};
var onGameStart = function() {
	if (typeof GameServer.onGameStart === 'function') GameServer.onGameStart();
};
var onPlayerAction = function(playerId, action, data) {
	if (typeof GameServer.onPlayerAction === 'function') GameServer.onPlayerAction(playerId, action, data);
};
var onGameEnd = function(results) {
	if (typeof GameServer.onGameEnd === 'function') GameServer.onGameEnd(results);
};
var onRestore = function(state) {
	if (typeof GameServer.onRestore === 'function') GameServer.onRestore(state);
};
var onTick = function(deltaTime) {
	if (typeof GameServer.onTick === 'function') GameServer.onTick(deltaTime);
};
var onTimer = function(timerId) {
	if (typeof GameServer.onTimer === 'function') GameServer.onTimer(timerId);
};
`
}

// safeExport safely exports a goja value, handling undefined/null.
func (e *GojaEngine) safeExport(v goja.Value) any {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	return v.Export()
}

// SetOnGameEnd sets the callback for game end events.
func (e *GojaEngine) SetOnGameEnd(fn func(roomID string, results any)) {
	e.onGameEnd = fn
}

// SetTimerCallback sets the callback invoked when a JS timer fires.
// The callback should route the timerID back to the owning actor's inbox
// to maintain the single-goroutine execution contract.
func (e *GojaEngine) SetTimerCallback(fn func(timerID int)) {
	e.onTimerCallback = fn
}
