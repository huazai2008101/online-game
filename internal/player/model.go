package player

import (
	"time"
)

type Player struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	GameID    uint      `gorm:"index" json:"game_id"`
	Nickname  string    `gorm:"size:50" json:"nickname"`
	Level     int       `gorm:"default:1" json:"level"`
	Exp       int64     `gorm:"default:0" json:"exp"`
	Score     int64     `gorm:"default:0" json:"score"`
	Status    int       `gorm:"default:1" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PlayerStats struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	PlayerID   uint      `gorm:"index" json:"player_id"`
	GamesPlayed int      `json:"games_played"`
	GamesWon   int       `json:"games_won"`
	TotalScore int64     `json:"total_score"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Player) TableName() string { return "players" }
func (PlayerStats) TableName() string { return "player_stats" }
