package payment

import (
	"time"
)

// Order represents a payment order
type Order struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	OrderNo     string    `gorm:"uniqueIndex;size:50" json:"order_no"`
	UserID      uint      `gorm:"index" json:"user_id"`
	ProductType string    `gorm:"size:20" json:"product_type"`
	ProductID   string    `gorm:"size:50" json:"product_id"`
	Amount      int64     `json:"amount"`
	Currency    string    `gorm:"size:10;default:USD" json:"currency"`
	Status      string    `gorm:"size:20;default:pending" json:"status"`
	PaymentMethod string  `gorm:"size:20" json:"payment_method"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Score represents user score/points
type Score struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex" json:"user_id"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ScoreLog represents score transaction log
type ScoreLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Type      string    `gorm:"size:20" json:"type"` // earn, spend, refund
	Amount    int64     `json:"amount"`
	Balance   int64     `json:"balance"`
	OrderID   *uint     `json:"order_id"`
	Reason    string    `gorm:"type:text" json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

func (Order) TableName() string { return "orders" }
func (Score) TableName() string { return "scores" }
func (ScoreLog) TableName() string { return "score_logs" }
