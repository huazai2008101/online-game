package game

import (
	"time"

	"gorm.io/gorm"
)

// Game represents a game
type Game struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	OrgID       int64  `json:"org_id"`
	GameCode    string `gorm:"uniqueIndex;size:50" json:"game_code"`
	GameName    string `gorm:"size:100" json:"game_name"`
	GameType    string `gorm:"size:20" json:"game_type"`
	GameIcon    string `gorm:"size:255" json:"game_icon,omitempty"`
	GameCover   string `gorm:"size:255" json:"game_cover,omitempty"`
	GameConfig  string `gorm:"type:text" json:"game_config,omitempty"` // JSON string
	Status      int    `gorm:"default:1" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// GameVersion represents a version of a game
type GameVersion struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	GameID      uint   `json:"game_id"`
	Version     string `gorm:"size:20" json:"version"`
	ScriptType  string `gorm:"size:20" json:"script_type"` // js or wasm
	ScriptPath  string `gorm:"size:255" json:"script_path"`
	ScriptHash  string `gorm:"size:64" json:"script_hash"`
	Description string `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   int64  `json:"created_by"`
}

// GameRoom represents a game room
type GameRoom struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	RoomID      string    `gorm:"uniqueIndex;size:50" json:"room_id"`
	GameID      uint      `json:"game_id"`
	RoomName    string    `gorm:"size:100" json:"room_name"`
	MaxPlayers  int       `json:"max_players"`
	PlayerCount int       `gorm:"default:0" json:"player_count"`
	Status      string    `gorm:"size:20;default:waiting" json:"status"` // waiting, playing, closed
	Config      string    `gorm:"type:text" json:"config,omitempty"` // JSON string
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
}

// GameSession represents a game session
type GameSession struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	RoomID        string    `gorm:"size:50;index" json:"room_id"`
	GameID        uint      `json:"game_id"`
	StartTime     time.Time `json:"start_time"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	Duration      int       `json:"duration"` // seconds
	Status        string    `gorm:"size:20" json:"status"` // running, completed, cancelled
	FinalState    string    `gorm:"type:text" json:"final_state,omitempty"` // JSON string
	CreatedAt     time.Time `json:"created_at"`
}

// TableName specifies the table name for Game
func (Game) TableName() string {
	return "games"
}

// TableName specifies the table name for GameVersion
func (GameVersion) TableName() string {
	return "game_versions"
}

// TableName specifies the table name for GameRoom
func (GameRoom) TableName() string {
	return "game_rooms"
}

// TableName specifies the table name for GameSession
func (GameSession) TableName() string {
	return "game_sessions"
}
