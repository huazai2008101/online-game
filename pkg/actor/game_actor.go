package actor

import (
	"context"
	"fmt"
	"sync"
)

// GameActor represents a running game instance
type GameActor struct {
	BaseActor
	gameID    string
	roomID    string
	state     map[string]any
	players   map[string]*PlayerActor
	isRunning bool
	tickCount int64
	mu        sync.RWMutex
}

// NewGameActor creates a new GameActor
func NewGameActor(gameID, roomID string) *GameActor {
	ctx, cancel := context.WithCancel(context.Background())
	return &GameActor{
		BaseActor: BaseActor{
			id:        fmt.Sprintf("game:%s:%s", gameID, roomID),
			actorType: "game",
			inbox:     make(chan Message, 100),
			ctx:       ctx,
			cancel:    cancel,
		},
		gameID:    gameID,
		roomID:    roomID,
		state:     make(map[string]any),
		players:   make(map[string]*PlayerActor),
		isRunning: false,
	}
}

// Receive handles incoming messages for GameActor
func (a *GameActor) Receive(ctx context.Context, msg Message) error {
	switch m := msg.(type) {
	case *GameStartMessage:
		return a.handleGameStart(ctx, m)
	case *GameStopMessage:
		return a.handleGameStop(ctx, m)
	case *GameTickMessage:
		return a.handleGameTick(ctx, m)
	case *PlayerJoinMessage:
		return a.handlePlayerJoin(ctx, m)
	case *PlayerLeaveMessage:
		return a.handlePlayerLeave(ctx, m)
	case *PlayerActionMessage:
		return a.handlePlayerAction(ctx, m)
	case *GameStateUpdateMessage:
		return a.handleGameStateUpdate(ctx, m)
	case *PingMessage:
		return a.handlePing(ctx, m)
	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
}

// handleGameStart starts the game
func (a *GameActor) handleGameStart(ctx context.Context, msg *GameStartMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return fmt.Errorf("game %s is already running", a.gameID)
	}

	a.isRunning = true
	a.tickCount = 0

	// Notify all players
	for playerID := range a.players {
		_ = a.Send(&PrivateMessage{
			PlayerID: playerID,
			Message: map[string]any{
				"type":      "game_start",
				"game_id":   a.gameID,
				"room_id":   a.roomID,
				"timestamp": msg.Timestamp,
			},
		})
	}

	return nil
}

// handleGameStop stops the game
func (a *GameActor) handleGameStop(ctx context.Context, msg *GameStopMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return fmt.Errorf("game %s is not running", a.gameID)
	}

	a.isRunning = false

	// Notify all players
	for playerID := range a.players {
		_ = a.Send(&PrivateMessage{
			PlayerID: playerID,
			Message: map[string]any{
				"type":    "game_stop",
				"reason":  msg.Reason,
				"game_id": a.gameID,
			},
		})
	}

	return nil
}

// handleGameTick processes a game tick
func (a *GameActor) handleGameTick(ctx context.Context, msg *GameTickMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return nil
	}

	a.tickCount = msg.TickCount

	// Process game logic for this tick
	// This would typically involve:
	// 1. Processing player inputs
	// 2. Updating game state
	// 3. Checking win conditions
	// 4. Sending state updates to players

	return nil
}

// handlePlayerJoin adds a player to the game
func (a *GameActor) handlePlayerJoin(ctx context.Context, msg *PlayerJoinMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.players[msg.PlayerID]; exists {
		return fmt.Errorf("player %s already in game", msg.PlayerID)
	}

	// Create player actor for this game
	playerActor := GetPlayerActor(msg.PlayerID)
	a.players[msg.PlayerID] = playerActor

	// Send welcome message
	_ = a.Send(&PrivateMessage{
		PlayerID: msg.PlayerID,
		Message: map[string]any{
			"type":      "welcome",
			"game_id":   a.gameID,
			"room_id":   a.roomID,
			"player_id": msg.PlayerID,
		},
	})

	return nil
}

// handlePlayerLeave removes a player from the game
func (a *GameActor) handlePlayerLeave(ctx context.Context, msg *PlayerLeaveMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	player, exists := a.players[msg.PlayerID]
	if !exists {
		return fmt.Errorf("player %s not in game", msg.PlayerID)
	}

	PutPlayerActor(player)
	delete(a.players, msg.PlayerID)

	return nil
}

// handlePlayerAction processes a player action
func (a *GameActor) handlePlayerAction(ctx context.Context, msg *PlayerActionMessage) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if _, exists := a.players[msg.PlayerID]; !exists {
		return fmt.Errorf("player %s not in game", msg.PlayerID)
	}

	// Process the player action
	// This would typically involve:
	// 1. Validating the action
	// 2. Updating game state
	// 3. Broadcasting the action result

	return nil
}

// handleGameStateUpdate handles game state updates
func (a *GameActor) handleGameStateUpdate(ctx context.Context, msg *GameStateUpdateMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state["last_update"] = msg.State
	a.state["tick"] = msg.Tick

	return nil
}

// handlePing responds to a ping message
func (a *GameActor) handlePing(ctx context.Context, msg *PingMessage) error {
	return nil
}

// GetState returns the current game state
func (a *GameActor) GetState() map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()

	state := make(map[string]any, len(a.state))
	for k, v := range a.state {
		state[k] = v
	}
	return state
}

// GetPlayerCount returns the number of players in the game
func (a *GameActor) GetPlayerCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.players)
}

// IsRunning returns whether the game is currently running
func (a *GameActor) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isRunning
}

// Reset resets the game actor state
func (a *GameActor) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state = make(map[string]any)
	a.players = make(map[string]*PlayerActor)
	a.isRunning = false
	a.tickCount = 0
}
