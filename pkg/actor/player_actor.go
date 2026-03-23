package actor

import (
	"context"
	"fmt"
	"sync"
)

// PlayerActor represents a player in the system
type PlayerActor struct {
	BaseActor
	playerID string
	nickname string
	avatar   string
	state    map[string]any
	isReady  bool
	mu       sync.RWMutex
}

// NewPlayerActor creates a new PlayerActor
func NewPlayerActor(playerID string) *PlayerActor {
	ctx, cancel := context.WithCancel(context.Background())
	return &PlayerActor{
		BaseActor: BaseActor{
			id:        fmt.Sprintf("player:%s", playerID),
			actorType: "player",
			inbox:     make(chan Message, 50),
			ctx:       ctx,
			cancel:    cancel,
		},
		playerID: playerID,
		state:    make(map[string]any),
		isReady:  false,
	}
}

// Receive handles incoming messages for PlayerActor
func (a *PlayerActor) Receive(ctx context.Context, msg Message) error {
	switch m := msg.(type) {
	case *PrivateMessage:
		return a.handlePrivateMessage(ctx, m)
	case *PlayerStateMessage:
		return a.handlePlayerState(ctx, m)
	case *PingMessage:
		return a.handlePing(ctx, m)
	case *GameStartMessage:
		return a.handleGameStart(ctx, m)
	case *GameStopMessage:
		return a.handleGameStop(ctx, m)
	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
}

// handlePrivateMessage handles private messages to the player
func (a *PlayerActor) handlePrivateMessage(ctx context.Context, msg *PrivateMessage) error {
	// Store the message for the player to retrieve
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state["last_message"] = msg.Message
	return nil
}

// handlePlayerState updates player state
func (a *PlayerActor) handlePlayerState(ctx context.Context, msg *PlayerStateMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for k, v := range msg.State {
		a.state[k] = v
	}
	return nil
}

// handlePing responds to a ping message
func (a *PlayerActor) handlePing(ctx context.Context, msg *PingMessage) error {
	// Player is alive
	return nil
}

// handleGameStart handles game start notification
func (a *PlayerActor) handleGameStart(ctx context.Context, msg *GameStartMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state["current_game"] = msg.GameID
	a.state["current_room"] = msg.RoomID
	return nil
}

// handleGameStop handles game stop notification
func (a *PlayerActor) handleGameStop(ctx context.Context, msg *GameStopMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.state, "current_game")
	delete(a.state, "current_room")
	return nil
}

// GetState returns the current player state
func (a *PlayerActor) GetState() map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()

	state := make(map[string]any, len(a.state))
	for k, v := range a.state {
		state[k] = v
	}
	return state
}

// SetNickname sets the player's nickname
func (a *PlayerActor) SetNickname(nickname string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nickname = nickname
}

// GetNickname returns the player's nickname
func (a *PlayerActor) GetNickname() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.nickname
}

// SetAvatar sets the player's avatar
func (a *PlayerActor) SetAvatar(avatar string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.avatar = avatar
}

// GetAvatar returns the player's avatar
func (a *PlayerActor) GetAvatar() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.avatar
}

// SetReady sets the player's ready state
func (a *PlayerActor) SetReady(ready bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.isReady = ready
}

// IsReady returns whether the player is ready
func (a *PlayerActor) IsReady() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isReady
}

// Reset resets the player actor state
func (a *PlayerActor) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state = make(map[string]any)
	a.isReady = false
}
