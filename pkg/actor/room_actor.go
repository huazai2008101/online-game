package actor

import (
	"context"
	"fmt"
	"sync"
)

// RoomActor represents a game room
type RoomActor struct {
	BaseActor
	roomID      string
	gameID      string
	players     []string
	maxPlayers  int
	isRunning   bool
	gameStarted bool
	mu          sync.RWMutex
}

// NewRoomActor creates a new RoomActor
func NewRoomActor(roomID, gameID string, maxPlayers int) *RoomActor {
	ctx, cancel := context.WithCancel(context.Background())
	return &RoomActor{
		BaseActor: BaseActor{
			id:        fmt.Sprintf("room:%s", roomID),
			actorType: "room",
			inbox:     make(chan Message, 50),
			ctx:       ctx,
			cancel:    cancel,
		},
		roomID:      roomID,
		gameID:      gameID,
		players:     make([]string, 0, maxPlayers),
		maxPlayers:  maxPlayers,
		isRunning:   false,
		gameStarted: false,
	}
}

// Receive handles incoming messages for RoomActor
func (a *RoomActor) Receive(ctx context.Context, msg Message) error {
	switch m := msg.(type) {
	case *RoomCreateMessage:
		return a.handleRoomCreate(ctx, m)
	case *RoomJoinMessage:
		return a.handleRoomJoin(ctx, m)
	case *RoomLeaveMessage:
		return a.handleRoomLeave(ctx, m)
	case *RoomCloseMessage:
		return a.handleRoomClose(ctx, m)
	case *BroadcastMessage:
		return a.handleBroadcast(ctx, m)
	case *PingMessage:
		return a.handlePing(ctx, m)
	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}
}

// handleRoomCreate creates a new room
func (a *RoomActor) handleRoomCreate(ctx context.Context, msg *RoomCreateMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return fmt.Errorf("room %s already exists", a.roomID)
	}

	a.roomID = msg.RoomID
	a.gameID = msg.GameID
	a.maxPlayers = msg.MaxPlayers
	a.isRunning = true

	return nil
}

// handleRoomJoin handles a player joining the room
func (a *RoomActor) handleRoomJoin(ctx context.Context, msg *RoomJoinMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if room is full
	if len(a.players) >= a.maxPlayers {
		return fmt.Errorf("room %s is full", a.roomID)
	}

	// Check if player already in room
	for _, p := range a.players {
		if p == msg.PlayerID {
			return fmt.Errorf("player %s already in room", msg.PlayerID)
		}
	}

	a.players = append(a.players, msg.PlayerID)

	// Broadcast player joined
	_ = a.Send(&BroadcastMessage{
		RoomID: a.roomID,
		Message: map[string]any{
			"type":      "player_joined",
			"player_id": msg.PlayerID,
			"room_id":   a.roomID,
		},
	})

	return nil
}

// handleRoomLeave handles a player leaving the room
func (a *RoomActor) handleRoomLeave(ctx context.Context, msg *RoomLeaveMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, p := range a.players {
		if p == msg.PlayerID {
			a.players = append(a.players[:i], a.players[i+1:]...)

			// Broadcast player left
			_ = a.Send(&BroadcastMessage{
				RoomID: a.roomID,
				Message: map[string]any{
					"type":      "player_left",
					"player_id": msg.PlayerID,
					"room_id":   a.roomID,
				},
			})

			return nil
		}
	}

	return fmt.Errorf("player %s not in room", msg.PlayerID)
}

// handleRoomClose closes the room
func (a *RoomActor) handleRoomClose(ctx context.Context, msg *RoomCloseMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.isRunning = false
	a.gameStarted = false

	// Broadcast room closed
	_ = a.Send(&BroadcastMessage{
		RoomID: a.roomID,
		Message: map[string]any{
			"type":    "room_closed",
			"room_id": a.roomID,
			"reason":  msg.Reason,
		},
	})

	return nil
}

// handleBroadcast broadcasts a message to all players in the room
func (a *RoomActor) handleBroadcast(ctx context.Context, msg *BroadcastMessage) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// This would typically send the message to all connected players
	// For now, we just acknowledge the broadcast
	return nil
}

// handlePing responds to a ping message
func (a *RoomActor) handlePing(ctx context.Context, msg *PingMessage) error {
	return nil
}

// GetPlayers returns the list of players in the room
func (a *RoomActor) GetPlayers() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	players := make([]string, len(a.players))
	copy(players, a.players)
	return players
}

// GetPlayerCount returns the number of players in the room
func (a *RoomActor) GetPlayerCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.players)
}

// IsFull returns whether the room is full
func (a *RoomActor) IsFull() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.players) >= a.maxPlayers
}

// IsRunning returns whether the room is running
func (a *RoomActor) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isRunning
}

// Reset resets the room actor state
func (a *RoomActor) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.players = make([]string, 0, a.maxPlayers)
	a.isRunning = false
	a.gameStarted = false
}
