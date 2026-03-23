package guild

import (
	"time"
)

type Guild struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	OrgID       int64     `json:"org_id"`
	GuildName   string    `gorm:"size:100" json:"guild_name"`
	LeaderID    uint      `json:"leader_id"`
	Level       int       `gorm:"default:1" json:"level"`
	Exp         int64     `json:"exp"`
	MemberCount int       `gorm:"default:1" json:"member_count"`
	MaxMembers  int       `gorm:"default:50" json:"max_members"`
	Status      int       `gorm:"default:1" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type GuildMember struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GuildID   uint      `gorm:"index" json:"guild_id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Role      string    `gorm:"size:20" json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

func (Guild) TableName() string { return "guilds" }
func (GuildMember) TableName() string { return "guild_members" }
