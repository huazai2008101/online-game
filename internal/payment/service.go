package payment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"online-game/pkg/apperror"
)

// TransactionalRepository defines transactional operations
type TransactionalRepository interface {
	WithTx(tx *gorm.DB) TransactionalRepository
}

// OrderRepository defines the interface for payment data operations
type OrderRepository interface {
	CreateOrder(order *Order) error
	GetOrderByID(id uint) (*Order, error)
	GetOrderByNo(orderNo string) (*Order, error)
	UpdateOrder(order *Order) error
	ListOrders(userID uint, offset, limit int) ([]*Order, int64, error)
	ListOrdersByStatus(status string, offset, limit int) ([]*Order, int64, error)
}

// ScoreRepository defines the interface for score data operations
type ScoreRepository interface {
	GetOrCreateScore(userID uint) (*Score, error)
	UpdateScore(score *Score) error
	UpdateScoreWithTx(tx *gorm.DB, score *Score) error
	CreateScoreLog(log *ScoreLog) error
	CreateScoreLogWithTx(tx *gorm.DB, log *ScoreLog) error
	GetScoreLogs(userID uint, offset, limit int) ([]*ScoreLog, int64, error)
	TransferScore(fromUserID, toUserID uint, amount int64, reason string) error
	BeginTx() *gorm.DB
}

// Service provides payment business logic
type Service struct {
	orderRepo OrderRepository
	scoreRepo ScoreRepository
}

// NewService creates a new payment service
func NewService(orderRepo OrderRepository, scoreRepo ScoreRepository) *Service {
	return &Service{
		orderRepo: orderRepo,
		scoreRepo: scoreRepo,
	}
}

// GenerateOrderNo generates a unique order number
func GenerateOrderNo() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	timestamp := time.Now().Unix()
	return fmt.Sprintf("ORD%d%s", timestamp, hex.EncodeToString(b)[:8].ToUpper()), nil
}

// Order operations

// CreateOrder creates a new payment order
func (s *Service) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
	if req.Amount <= 0 {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "amount", "message": "金额必须大于0"})
	}

	orderNo, err := GenerateOrderNo()
	if err != nil {
		return nil, apperror.ErrInternalServer.WithMessage("生成订单号失败")
	}

	order := &Order{
		OrderNo:       orderNo,
		UserID:        req.UserID,
		ProductType:   req.ProductType,
		ProductID:     req.ProductID,
		Amount:        req.Amount,
		Currency:      "USD",
		Status:        "pending",
		PaymentMethod: req.PaymentMethod,
	}

	if err := s.orderRepo.CreateOrder(order); err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}

	return order, nil
}

// GetOrder retrieves an order by ID
func (s *Service) GetOrder(ctx context.Context, id uint) (*Order, error) {
	return s.orderRepo.GetOrderByID(id)
}

// GetOrderByNo retrieves an order by order number
func (s *Service) GetOrderByNo(ctx context.Context, orderNo string) (*Order, error) {
	return s.orderRepo.GetOrderByNo(orderNo)
}

// ListOrders lists orders with pagination
func (s *Service) ListOrders(ctx context.Context, userID uint, page, pageSize int) ([]*Order, int64, error) {
	offset := (page - 1) * pageSize
	return s.orderRepo.ListOrders(userID, offset, pageSize)
}

// ProcessPayment processes a payment for an order
func (s *Service) ProcessPayment(ctx context.Context, orderID uint, paymentData map[string]interface{}) error {
	order, err := s.orderRepo.GetOrderByID(orderID)
	if err != nil {
		return apperror.OrderNotFound()
	}

	if order.Status == "paid" || order.Status == "completed" {
		return apperror.InvalidState("订单已支付")
	}

	// In a real implementation, this would integrate with payment gateways
	// For now, we'll simulate successful payment
	order.Status = "paid"

	if err := s.orderRepo.UpdateOrder(order); err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	// Add score to user's account
	if order.ProductType == "score" || order.ProductType == "coins" {
		_, err := s.AddScore(ctx, order.UserID, order.Amount, &order.ID, fmt.Sprintf("订单支付: %s", order.OrderNo))
		if err != nil {
			return apperror.ErrInternalServer.WithData(err.Error())
		}
	}

	return nil
}

// RefundOrder processes a refund for an order
func (s *Service) RefundOrder(ctx context.Context, orderID uint, reason string) error {
	order, err := s.orderRepo.GetOrderByID(orderID)
	if err != nil {
		return apperror.OrderNotFound()
	}

	if order.Status != "paid" && order.Status != "completed" {
		return apperror.InvalidState("订单状态不允许退款")
	}

	order.Status = "refunded"

	if err := s.orderRepo.UpdateOrder(order); err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	// Deduct score back
	if order.ProductType == "score" || order.ProductType == "coins" {
		_, err := s.DeductScore(ctx, order.UserID, order.Amount, &order.ID, fmt.Sprintf("订单退款: %s. 原因: %s", order.OrderNo, reason))
		if err != nil {
			return apperror.ErrInternalServer.WithData(err.Error())
		}
	}

	return nil
}

// Score operations

// GetScore retrieves user's score balance
func (s *Service) GetScore(ctx context.Context, userID uint) (*Score, error) {
	return s.scoreRepo.GetOrCreateScore(userID)
}

// AddScore adds score to user's account
func (s *Service) AddScore(ctx context.Context, userID uint, amount int64, orderID *uint, reason string) (*Score, error) {
	if amount <= 0 {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "amount", "message": "金额必须大于0"})
	}

	score, err := s.scoreRepo.GetOrCreateScore(userID)
	if err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}

	oldBalance := score.Balance
	score.Balance += amount

	if err := s.scoreRepo.UpdateScore(score); err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}

	// Create log
	log := &ScoreLog{
		UserID:  userID,
		Type:    "earn",
		Amount:  amount,
		Balance: score.Balance,
		OrderID: orderID,
		Reason:  reason,
	}

	if err := s.scoreRepo.CreateScoreLog(log); err != nil {
		// Log error but don't fail the transaction
		fmt.Printf("failed to create score log: %v", err)
	}

	return score, nil
}

// DeductScore deducts score from user's account
func (s *Service) DeductScore(ctx context.Context, userID uint, amount int64, orderID *uint, reason string) (*Score, error) {
	if amount <= 0 {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "amount", "message": "金额必须大于0"})
	}

	score, err := s.scoreRepo.GetOrCreateScore(userID)
	if err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}

	if score.Balance < amount {
		return nil, apperror.InsufficientBalance("积分余额不足")
	}

	oldBalance := score.Balance
	score.Balance -= amount

	if err := s.scoreRepo.UpdateScore(score); err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}

	// Create log
	log := &ScoreLog{
		UserID:  userID,
		Type:    "spend",
		Amount:  -amount,
		Balance: score.Balance,
		OrderID: orderID,
		Reason:  reason,
	}

	if err := s.scoreRepo.CreateScoreLog(log); err != nil {
		fmt.Printf("failed to create score log: %v", err)
	}

	return score, nil
}

// ConsumeScore consumes score from user's account (alias for DeductScore)
func (s *Service) ConsumeScore(ctx context.Context, userID uint, amount int64, orderID *uint, reason string) (*Score, error) {
	return s.DeductScore(ctx, userID, amount, orderID, reason)
}

// RechargeScore recharges score to user's account (alias for AddScore)
func (s *Service) RechargeScore(ctx context.Context, userID uint, amount int64, orderID *uint, reason string) (*Score, error) {
	return s.AddScore(ctx, userID, amount, orderID, reason)
}

// TransferScore transfers score between users using a transaction
func (s *Service) TransferScore(ctx context.Context, fromUserID, toUserID uint, amount int64, reason string) error {
	if amount <= 0 {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "amount", "message": "金额必须大于0"})
	}

	if fromUserID == toUserID {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "to_user_id", "message": "不能转账给自己"})
	}

	// Use repository's transaction method
	return s.scoreRepo.TransferScore(fromUserID, toUserID, amount, reason)
}

// GetScoreLogs retrieves score transaction logs
func (s *Service) GetScoreLogs(ctx context.Context, userID uint, page, pageSize int) ([]*ScoreLog, int64, error) {
	offset := (page - 1) * pageSize
	return s.scoreRepo.GetScoreLogs(userID, offset, pageSize)
}

// Request types

// CreateOrderRequest represents the request to create an order
type CreateOrderRequest struct {
	UserID        uint   `json:"user_id" binding:"required"`
	ProductType   string `json:"product_type" binding:"required,oneof=score coins item vip"`
	ProductID     string `json:"product_id" binding:"required"`
	Amount        int64  `json:"amount" binding:"required,min=1"`
	PaymentMethod string `json:"payment_method" binding:"required,oneof=alipay wechat card score"`
	Currency      string `json:"currency" binding:"omitempty,oneof=USD CNY EUR"`
}

// ConsumeScoreRequest represents the request to consume score
type ConsumeScoreRequest struct {
	UserID  uint   `json:"user_id" binding:"required"`
	Amount  int64  `json:"amount" binding:"required,min=1"`
	OrderID *uint  `json:"order_id"`
	Reason  string `json:"reason" binding:"required"`
}

// RechargeScoreRequest represents the request to recharge score
type RechargeScoreRequest struct {
	UserID  uint   `json:"user_id" binding:"required"`
	Amount  int64  `json:"amount" binding:"required,min=1"`
	OrderID *uint  `json:"order_id"`
	Reason  string `json:"reason" binding:"required"`
}

// TransferScoreRequest represents the request to transfer score
type TransferScoreRequest struct {
	FromUserID uint   `json:"from_user_id" binding:"required"`
	ToUserID   uint   `json:"to_user_id" binding:"required"`
	Amount     int64  `json:"amount" binding:"required,min=1"`
	Reason     string `json:"reason"`
}

// Repository implementations

// OrderRepositoryImpl implements OrderRepository
type OrderRepositoryImpl struct {
	db *gorm.DB
}

// NewOrderRepositoryImpl creates a new order repository
func NewOrderRepositoryImpl(db *gorm.DB) OrderRepository {
	return &OrderRepositoryImpl{db: db}
}

func (r *OrderRepositoryImpl) CreateOrder(order *Order) error {
	err := r.db.Create(order).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

func (r *OrderRepositoryImpl) GetOrderByID(id uint) (*Order, error) {
	var order Order
	err := r.db.First(&order, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.OrderNotFound()
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &order, nil
}

func (r *OrderRepositoryImpl) GetOrderByNo(orderNo string) (*Order, error) {
	var order Order
	err := r.db.Where("order_no = ?", orderNo).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.OrderNotFound()
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &order, nil
}

func (r *OrderRepositoryImpl) UpdateOrder(order *Order) error {
	err := r.db.Save(order).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

func (r *OrderRepositoryImpl) ListOrders(userID uint, offset, limit int) ([]*Order, int64, error) {
	var orders []*Order
	var total int64

	query := r.db.Model(&Order{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}

	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&orders).Error
	if err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}
	return orders, total, nil
}

func (r *OrderRepositoryImpl) ListOrdersByStatus(status string, offset, limit int) ([]*Order, int64, error) {
	var orders []*Order
	var total int64

	query := r.db.Model(&Order{}).Where("status = ?", status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}

	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&orders).Error
	if err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}
	return orders, total, nil
}

// ScoreRepositoryImpl implements ScoreRepository
type ScoreRepositoryImpl struct {
	db *gorm.DB
}

// NewScoreRepositoryImpl creates a new score repository
func NewScoreRepositoryImpl(db *gorm.DB) ScoreRepository {
	return &ScoreRepositoryImpl{db: db}
}

func (r *ScoreRepositoryImpl) GetOrCreateScore(userID uint) (*Score, error) {
	var score Score
	err := r.db.Where("user_id = ?", userID).First(&score).Error
	if err == nil {
		return &score, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}

	score = Score{UserID: userID, Balance: 0}
	err = r.db.Create(&score).Error
	if err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &score, nil
}

func (r *ScoreRepositoryImpl) UpdateScore(score *Score) error {
	err := r.db.Save(score).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

func (r *ScoreRepositoryImpl) CreateScoreLog(log *ScoreLog) error {
	err := r.db.Create(log).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

func (r *ScoreRepositoryImpl) GetScoreLogs(userID uint, offset, limit int) ([]*ScoreLog, int64, error) {
	var logs []*ScoreLog
	var total int64

	query := r.db.Model(&ScoreLog{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}

	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&logs).Error
	if err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}
	return logs, total, nil
}

// UpdateScoreWithTx updates score within a transaction
func (r *ScoreRepositoryImpl) UpdateScoreWithTx(tx *gorm.DB, score *Score) error {
	return tx.Save(score).Error
}

// CreateScoreLogWithTx creates a score log within a transaction
func (r *ScoreRepositoryImpl) CreateScoreLogWithTx(tx *gorm.DB, log *ScoreLog) error {
	return tx.Create(log).Error
}

// BeginTx begins a new transaction
func (r *ScoreRepositoryImpl) BeginTx() *gorm.DB {
	return r.db.Begin()
}

// TransferScore transfers score between users in a transaction
func (r *ScoreRepositoryImpl) TransferScore(fromUserID, toUserID uint, amount int64, reason string) error {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// Lock and get sender's score
	var fromScore Score
	if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Where("user_id = ?", fromUserID).First(&fromScore).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return apperror.InsufficientBalance("积分余额不足")
		}
		tx.Rollback()
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	// Lock and get receiver's score
	var toScore Score
	if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Where("user_id = ?", toUserID).First(&toScore).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create receiver's score if not exists
			toScore = Score{UserID: toUserID, Balance: 0}
			if err := tx.Create(&toScore).Error; err != nil {
				tx.Rollback()
				return apperror.ErrInternalServer.WithData(err.Error())
			}
		} else {
			tx.Rollback()
			return apperror.ErrInternalServer.WithData(err.Error())
		}
	}

	// Check balance
	if fromScore.Balance < amount {
		tx.Rollback()
		return apperror.InsufficientBalance("积分余额不足")
	}

	// Transfer
	fromScore.Balance -= amount
	toScore.Balance += amount

	if err := tx.Save(&fromScore).Error; err != nil {
		tx.Rollback()
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	if err := tx.Save(&toScore).Error; err != nil {
		tx.Rollback()
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	// Create logs
	fromLog := &ScoreLog{
		UserID:  fromUserID,
		Type:    "transfer_out",
		Amount:  -amount,
		Balance: fromScore.Balance,
		Reason:  fmt.Sprintf("转账给用户%d: %s", toUserID, reason),
	}
	if err := tx.Create(fromLog).Error; err != nil {
		tx.Rollback()
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	toLog := &ScoreLog{
		UserID:  toUserID,
		Type:    "transfer_in",
		Amount:  amount,
		Balance: toScore.Balance,
		Reason:  fmt.Sprintf("从用户%d转账: %s", fromUserID, reason),
	}
	if err := tx.Create(toLog).Error; err != nil {
		tx.Rollback()
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}

	return nil
}
