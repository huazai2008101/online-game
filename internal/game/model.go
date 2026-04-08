package game

import (
	"time"

	"gorm.io/gorm"
)

// Game represents a game registered on the platform.
type Game struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	GameCode    string         `gorm:"uniqueIndex;size:50;not null" json:"game_code"`
	GameName    string         `gorm:"size:100;not null" json:"game_name"`
	GameType    string         `gorm:"size:20;default:turn-based" json:"game_type"` // realtime, turn-based
	Description string         `gorm:"type:text" json:"description,omitempty"`
	GameIcon    string         `gorm:"size:255" json:"game_icon,omitempty"`
	GameCover   string         `gorm:"size:255" json:"game_cover,omitempty"`
	MinPlayers  int            `gorm:"default:2" json:"min_players"`
	MaxPlayers  int            `gorm:"default:10" json:"max_players"`
	Status      string         `gorm:"size:20;default:draft;not null" json:"status"` // draft, published, offline
	SortOrder   int            `gorm:"default:0" json:"sort_order"`
	Config      string         `gorm:"type:text" json:"config,omitempty"`             // JSON config
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// GameVersion tracks uploaded game packages.
type GameVersion struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	GameID      uint      `gorm:"index;not null" json:"game_id"`
	Version     string    `gorm:"size:20;not null" json:"version"`
	ScriptType  string    `gorm:"size:10;default:js" json:"script_type"`
	PackagePath string    `gorm:"size:500" json:"package_path"`
	PackageHash string    `gorm:"size:64" json:"package_hash"`
	PackageSize int64     `json:"package_size"`
	EntryScript string    `gorm:"size:255" json:"entry_script"`
	Status      string    `gorm:"size:20;default:active" json:"status"` // active, deprecated
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   uint      `json:"created_by"`
}

// GameRoom represents a game room.
type GameRoom struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	RoomID      string         `gorm:"uniqueIndex;size:36;not null" json:"room_id"`
	GameID      uint           `gorm:"index;not null" json:"game_id"`
	GameVersion string         `gorm:"size:20" json:"game_version,omitempty"`
	RoomName    string         `gorm:"size:100" json:"room_name"`
	OwnerID     string         `gorm:"size:36" json:"owner_id"`
	MaxPlayers  int            `gorm:"default:4" json:"max_players"`
	Status      string         `gorm:"size:20;default:waiting;not null" json:"status"` // waiting, playing, closed
	Config      string         `gorm:"type:text" json:"config,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	ClosedAt    *time.Time     `json:"closed_at,omitempty"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// GameSession records a completed game.
type GameSession struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	RoomID     string    `gorm:"index;size:36;not null" json:"room_id"`
	GameID     uint      `gorm:"index;not null" json:"game_id"`
	Status     string    `gorm:"size:20;default:running" json:"status"` // running, completed, cancelled
	StartTime  time.Time `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Duration   int       `json:"duration"` // seconds
	FinalState string    `gorm:"type:text" json:"final_state,omitempty"` // JSON
	Results    string    `gorm:"type:text" json:"results,omitempty"`     // JSON
}

// RoomPlayer tracks players in a room.
type RoomPlayer struct {
	ID       uint       `gorm:"primaryKey" json:"id"`
	RoomID   string     `gorm:"index;size:36;not null" json:"room_id"`
	PlayerID string     `gorm:"index;size:36;not null" json:"player_id"`
	Nickname string     `gorm:"size:50" json:"nickname"`
	SeatIdx  int        `json:"seat_index"`
	Status   string     `gorm:"size:20;default:waiting" json:"status"` // waiting, ready, playing, left
	Score    int        `gorm:"default:0" json:"score"`
	JoinedAt time.Time  `json:"joined_at"`
	LeftAt   *time.Time `json:"left_at,omitempty"`
}

// CreateRoomRequest is the input for creating a room.
type CreateRoomRequest struct {
	GameID     uint   `json:"game_id" binding:"required"`
	RoomName   string `json:"room_name" binding:"max=100"`
	MaxPlayers int    `json:"max_players" binding:"omitempty,min=2,max=100"`
}

// GameListQuery is the query for listing games.
type GameListQuery struct {
	GameType string `form:"game_type"`
	Status   string `form:"status"`
	Search   string `form:"search"`
}
