package actor

// ==================== Game Related Messages ====================

// GameStartMessage signals a game to start
type GameStartMessage struct {
	GameID    string
	RoomID    string
	Players   []string
	Timestamp int64
}

// GameTickMessage represents a game tick
type GameTickMessage struct {
	Timestamp int64
	TickCount int64
	DeltaTime float64
}

// GameStopMessage signals a game to stop
type GameStopMessage struct {
	Reason  string
	GameID  string
	RoomID  string
}

// GameStateUpdateMessage updates the game state
type GameStateUpdateMessage struct {
	GameID string
	RoomID string
	State  interface{}
	Tick   int64
}

// ==================== Player Related Messages ====================

// PlayerJoinMessage signals a player joining a game
type PlayerJoinMessage struct {
	PlayerID   string
	PlayerName string
	PlayerData map[string]interface{}
	GameID     string
	RoomID     string
}

// PlayerLeaveMessage signals a player leaving a game
type PlayerLeaveMessage struct {
	PlayerID string
	Reason   string
	GameID   string
	RoomID   string
}

// PlayerActionMessage represents a player action
type PlayerActionMessage struct {
	PlayerID string
	Action   string
	Data     interface{}
	GameID   string
	RoomID   string
	Timestamp int64
}

// PlayerStateMessage updates player state
type PlayerStateMessage struct {
	PlayerID string
	State    map[string]interface{}
}

// ==================== Room Related Messages ====================

// RoomCreateMessage creates a new room
type RoomCreateMessage struct {
	RoomID     string
	GameID     string
	MaxPlayers int
	Config     map[string]interface{}
}

// RoomJoinMessage joins a room
type RoomJoinMessage struct {
	RoomID   string
	PlayerID string
}

// RoomLeaveMessage leaves a room
type RoomLeaveMessage struct {
	RoomID   string
	PlayerID string
}

// RoomCloseMessage closes a room
type RoomCloseMessage struct {
	RoomID string
	Reason string
}

// ==================== System Messages ====================

// PingMessage is a health check message
type PingMessage struct {
	Timestamp int64
}

// PongMessage is the response to a ping
type PongMessage struct {
	Timestamp    int64
	OriginalTime int64
}

// ShutdownMessage signals the actor to shut down
type ShutdownMessage struct {
	Reason string
}

// ==================== Broadcast Messages ====================

// BroadcastMessage sends a message to all players in a room
type BroadcastMessage struct {
	RoomID    string
	Message   interface{}
	ExcludeID string
}

// PrivateMessage sends a message to a specific player
type PrivateMessage struct {
	PlayerID string
	Message  interface{}
}

// ==================== Engine Messages ====================

// EngineSwitchMessage switches the game engine
type EngineSwitchMessage struct {
	EngineType string
	Reason     string
}

// EngineExecuteMessage executes code on the engine
type EngineExecuteMessage struct {
	Method string
	Args   interface{}
}

// EngineStatsMessage requests engine statistics
type EngineStatsMessage struct {
	IncludeDetails bool
}
