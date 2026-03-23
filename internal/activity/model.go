package activity

import (
	"time"
)

type Activity struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	OrgID       int64     `json:"org_id"`
	Name        string    `gorm:"size:100" json:"name"`
	Type        string    `gorm:"size:20" json:"type"`
	Description string    `gorm:"type:text" json:"description"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Status      string    `gorm:"size:20" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type ActivityReward struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ActivityID uint      `json:"activity_id"`
	Name       string    `gorm:"size:100" json:"name"`
	RewardType string    `gorm:"size:20" json:"reward_type"`
	Amount     int64     `json:"amount"`
}

func (Activity) TableName() string { return "activities" }
func (ActivityReward) TableName() string { return "activity_rewards" }
