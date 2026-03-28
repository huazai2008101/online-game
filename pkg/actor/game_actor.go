package actor

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

// GameActor manages a single game room.
// One GameActor = one goroutine = one goja runtime = zero locks.
//
// All game state is accessed exclusively from the run goroutine.
// Messages arrive via the inbox channel and are processed sequentially.
type GameActor struct {
	*BaseActor

	// Identity
	gameID   string
	roomID   string
	gameCode string

	// Engine (owned exclusively by this actor's goroutine)
	engine EngineInterface

	// Room state (accessed only from actor goroutine — no locks needed)
	players    map[string]*PlayerState
	ownerID    string
	maxPlayers int
	isPlaying  bool

	// Injected dependencies
	hub     HubInterface
	roomMgr RoomManagerInterface

	// Stats
	tickCount atomic.Int64
	createdAt time.Time
}

// EngineInterface abstracts the JS engine for testability.
type EngineInterface interface {
	TriggerHook(hookName string, args ...any) (any, error)
	LoadScript(scriptContent string) error
	Destroy()
}

// HubInterface abstracts WebSocket message delivery.
type HubInterface interface {
	BroadcastToRoom(roomID string, event string, data any)
	SendTo(playerID string, event string, data any)
	SendExcept(roomID string, exceptPlayerID string, event string, data any)
}

// RoomManagerInterface abstracts room persistence.
type RoomManagerInterface interface {
	GetPlayerIDs(roomID string) []string
	GetConfig(roomID string) map[string]any
	UpdateRoomStatus(roomID string, status string)
	RecordGameResults(roomID string, results any)
}

// PlayerState holds per-player runtime state (no locks — accessed only from actor goroutine).
type PlayerState struct {
	ID       string
	Nickname string
	Avatar   string
	IsReady  bool
	IsOnline bool
	JoinedAt time.Time
	Data     map[string]any // game-specific player data
}

// GameActorConfig holds configuration for creating a GameActor.
type GameActorConfig struct {
	GameID     string
	RoomID     string
	GameCode   string
	MaxPlayers int
	InboxCap   int
	Hub        HubInterface
	RoomMgr    RoomManagerInterface
	Engine     EngineInterface
}

// NewGameActor creates a new game actor for a room.
func NewGameActor(cfg GameActorConfig) *GameActor {
	ga := &GameActor{
		gameID:     cfg.GameID,
		roomID:     cfg.RoomID,
		gameCode:   cfg.GameCode,
		engine:     cfg.Engine,
		players:    make(map[string]*PlayerState),
		maxPlayers: cfg.MaxPlayers,
		hub:        cfg.Hub,
		roomMgr:    cfg.RoomMgr,
		createdAt:  time.Now(),
	}

	actorID := fmt.Sprintf("game:%s:%s", cfg.GameID, cfg.RoomID)
	ga.BaseActor = NewBaseActor(actorID, ga.handleMessage, cfg.InboxCap)
	return ga
}

// handleMessage dispatches messages to the appropriate handler.
// This runs on the actor's single goroutine — no synchronization needed.
func (ga *GameActor) handleMessage(msg *Message) error {
	switch msg.Type {
	case MsgPlayerJoin:
		return ga.handlePlayerJoin(msg)
	case MsgPlayerLeave:
		return ga.handlePlayerLeave(msg)
	case MsgPlayerReady:
		return ga.handlePlayerReady(msg)
	case MsgPlayerAction:
		return ga.handlePlayerAction(msg)
	case MsgGameStart:
		return ga.handleGameStart(msg)
	case MsgGameTick:
		return ga.handleGameTick(msg)
	case MsgTimer:
		return ga.handleTimer(msg)
	case MsgRestore:
		return ga.handleRestore(msg)
	case MsgShutdown:
		return ga.handleShutdown(msg)
	default:
		return fmt.Errorf("unknown message type: %d", msg.Type)
	}
}

// handlePlayerJoin processes a player joining the room.
func (ga *GameActor) handlePlayerJoin(msg *Message) error {
	if ga.isPlaying {
		ga.hub.SendTo(msg.PlayerID, "error", map[string]any{
			"code": 46402, "message": "游戏进行中，无法加入",
		})
		return nil
	}

	if len(ga.players) >= ga.maxPlayers {
		ga.hub.SendTo(msg.PlayerID, "error", map[string]any{
			"code": 46901, "message": "房间已满",
		})
		return nil
	}

	if _, exists := ga.players[msg.PlayerID]; exists {
		return nil // already in room
	}

	// Extract player info
	data, ok := msg.Data.(*PlayerJoinData)
	if !ok {
		return fmt.Errorf("invalid PlayerJoinData for player %s", msg.PlayerID)
	}

	// Set owner if first player
	if len(ga.players) == 0 {
		ga.ownerID = msg.PlayerID
	}

	ga.players[msg.PlayerID] = &PlayerState{
		ID:       msg.PlayerID,
		Nickname: data.Nickname,
		Avatar:   data.Avatar,
		IsReady:  false,
		IsOnline: true,
		JoinedAt: time.Now(),
		Data:     data.Metadata,
	}

	// Notify JS engine
	if ga.engine != nil {
		ga.engine.TriggerHook("onPlayerJoin", msg.PlayerID, map[string]any{
			"nickname": data.Nickname,
			"avatar":   data.Avatar,
		})
	}

	// Broadcast to room
	ga.hub.BroadcastToRoom(ga.roomID, "player_join", map[string]any{
		"playerId":    msg.PlayerID,
		"nickname":    data.Nickname,
		"avatar":      data.Avatar,
		"playerCount": len(ga.players),
		"maxPlayers":  ga.maxPlayers,
	})

	slog.Debug("player joined room",
		"room_id", ga.roomID,
		"player_id", msg.PlayerID,
		"players", len(ga.players),
	)
	return nil
}

// handlePlayerLeave processes a player leaving the room.
func (ga *GameActor) handlePlayerLeave(msg *Message) error {
	player, exists := ga.players[msg.PlayerID]
	if !exists {
		return nil
	}

	reason := "voluntary"
	if data, ok := msg.Data.(*PlayerLeaveData); ok {
		reason = data.Reason
	}

	delete(ga.players, msg.PlayerID)

	// Notify JS engine
	if ga.engine != nil {
		ga.engine.TriggerHook("onPlayerLeave", msg.PlayerID, reason)
	}

	// Broadcast to room
	ga.hub.BroadcastToRoom(ga.roomID, "player_leave", map[string]any{
		"playerId": msg.PlayerID,
		"nickname": player.Nickname,
		"reason":   reason,
	})

	// If owner left, transfer ownership
	if ga.ownerID == msg.PlayerID && len(ga.players) > 0 {
		for pid := range ga.players {
			ga.ownerID = pid
			ga.hub.BroadcastToRoom(ga.roomID, "owner_change", map[string]any{
				"newOwnerId": pid,
			})
			break
		}
	}

	// If room empty, mark for shutdown
	if len(ga.players) == 0 && !ga.isPlaying {
		ga.roomMgr.UpdateRoomStatus(ga.roomID, "closed")
	}

	slog.Debug("player left room",
		"room_id", ga.roomID,
		"player_id", msg.PlayerID,
		"reason", reason,
		"remaining", len(ga.players),
	)
	return nil
}

// handlePlayerReady marks a player as ready.
func (ga *GameActor) handlePlayerReady(msg *Message) error {
	player, exists := ga.players[msg.PlayerID]
	if !exists {
		return nil
	}
	player.IsReady = true

	ga.hub.BroadcastToRoom(ga.roomID, "player_ready", map[string]any{
		"playerId": msg.PlayerID,
	})

	// Check if all players ready
	allReady := true
	for _, p := range ga.players {
		if !p.IsReady {
			allReady = false
			break
		}
	}

	if allReady {
		ga.hub.BroadcastToRoom(ga.roomID, "all_ready", map[string]any{
			"playerCount": len(ga.players),
		})
	}
	return nil
}

// handlePlayerAction is the core game loop: player action → goja execution → broadcast.
func (ga *GameActor) handlePlayerAction(msg *Message) error {
	if !ga.isPlaying {
		ga.hub.SendTo(msg.PlayerID, "error", map[string]any{
			"code": 46001, "message": "游戏尚未开始",
		})
		return nil
	}

	if _, exists := ga.players[msg.PlayerID]; !exists {
		return nil
	}

	// Delegate to JS engine — the goja runtime executes game logic
	if ga.engine != nil {
		_, err := ga.engine.TriggerHook("onPlayerAction", msg.PlayerID, msg.Action, msg.Data)
		if err != nil {
			slog.Error("engine execute error",
				"room_id", ga.roomID,
				"player_id", msg.PlayerID,
				"action", msg.Action,
				"error", err,
			)
			ga.hub.SendTo(msg.PlayerID, "error", map[string]any{
				"code":    47001,
				"message": "脚本执行错误",
				"detail":  err.Error(),
			})
		}
	}
	return nil
}

// handleGameStart starts the game.
func (ga *GameActor) handleGameStart(_ *Message) error {
	if ga.isPlaying {
		return nil
	}

	ga.isPlaying = true
	ga.roomMgr.UpdateRoomStatus(ga.roomID, "playing")

	if ga.engine != nil {
		ga.engine.TriggerHook("onGameStart")
	}

	ga.hub.BroadcastToRoom(ga.roomID, "game_start", map[string]any{
		"roomID": ga.roomID,
	})

	slog.Info("game started", "room_id", ga.roomID, "players", len(ga.players))
	return nil
}

// handleGameTick handles periodic game ticks for realtime games.
func (ga *GameActor) handleGameTick(msg *Message) error {
	if !ga.isPlaying {
		return nil
	}
	ga.tickCount.Add(1)

	if ga.engine != nil {
		deltaMs := float64(16)
		if d, ok := msg.Data.(float64); ok {
			deltaMs = d
		}
		ga.engine.TriggerHook("onTick", deltaMs)
	}
	return nil
}

// handleTimer processes a timer callback from the JS engine.
func (ga *GameActor) handleTimer(msg *Message) error {
	if td, ok := msg.Data.(*TimerData); ok && ga.engine != nil {
		ga.engine.TriggerHook("onTimer", td.ID)
	}
	return nil
}

// handleRestore restores game state (for reconnection).
func (ga *GameActor) handleRestore(msg *Message) error {
	if ga.engine != nil {
		ga.engine.TriggerHook("onRestore", msg.Data)
	}
	slog.Info("game state restored", "room_id", ga.roomID)
	return nil
}

// handleShutdown cleans up resources.
func (ga *GameActor) handleShutdown(_ *Message) error {
	if ga.isPlaying {
		if ga.engine != nil {
			ga.engine.TriggerHook("onGameEnd", map[string]any{
				"reason": "server_shutdown",
			})
		}
		ga.isPlaying = false
	}

	if ga.engine != nil {
		ga.engine.Destroy()
	}

	ga.roomMgr.UpdateRoomStatus(ga.roomID, "closed")
	slog.Info("game actor shutdown", "room_id", ga.roomID, "ticks", ga.tickCount.Load())
	return nil
}

// PlayerCount returns the number of players.
func (ga *GameActor) PlayerCount() int {
	return len(ga.players)
}

// RoomID returns the room ID.
func (ga *GameActor) RoomID() string { return ga.roomID }

// GameID returns the game ID.
func (ga *GameActor) GameID() string { return ga.gameID }
