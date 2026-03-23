package payment

import (
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateOrder(order *Order) error {
	return r.db.Create(order).Error
}

func (r *Repository) GetOrderByID(id uint) (*Order, error) {
	var order Order
	err := r.db.First(&order, id).Error
	return &order, err
}

func (r *Repository) GetOrderByNo(orderNo string) (*Order, error) {
	var order Order
	err := r.db.Where("order_no = ?", orderNo).First(&order).Error
	return &order, err
}

func (r *Repository) UpdateOrder(order *Order) error {
	return r.db.Save(order).Error
}

func (r *Repository) ListOrders(userID uint, offset, limit int) ([]*Order, int64, error) {
	var orders []*Order
	var total int64

	query := r.db.Model(&Order{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	query.Count(&total)
	err := query.Offset(offset).Limit(limit).Find(&orders).Error
	return orders, total, err
}

func (r *Repository) GetOrCreateScore(userID uint) (*Score, error) {
	var score Score
	err := r.db.Where("user_id = ?", userID).First(&score).Error
	if err == nil {
		return &score, nil
	}

	score = Score{UserID: userID, Balance: 0}
	err = r.db.Create(&score).Error
	return &score, err
}

func (r *Repository) UpdateScore(score *Score) error {
	return r.db.Save(score).Error
}

func (r *Repository) CreateScoreLog(log *ScoreLog) error {
	return r.db.Create(log).Error
}
