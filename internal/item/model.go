package item

import (
	"time"
)

type Item struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	OrgID       int64     `json:"org_id"`
	ItemCode    string    `gorm:"size:50" json:"item_code"`
	ItemName    string    `gorm:"size:100" json:"item_name"`
	ItemType    string    `gorm:"size:20" json:"item_type"`
	Price       int64     `json:"price"`
	Description string    `gorm:"type:text" json:"description"`
	Status      int       `gorm:"default:1" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type UserItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	ItemID    uint      `json:"item_id"`
	Quantity  int       `json:"quantity"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (Item) TableName() string { return "items" }
func (UserItem) TableName() string { return "user_items" }
