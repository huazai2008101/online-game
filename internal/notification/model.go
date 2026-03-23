package notification

import (
	"time"
)

type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Type      string    `gorm:"size:20" json:"type"`
	Title     string    `gorm:"size:100" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	Read      bool      `gorm:"default:false" json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

type Message struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Type      string    `gorm:"size:20" json:"type"`
	Title     string    `gorm:"size:100" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }
func (Message) TableName() string { return "messages" }
