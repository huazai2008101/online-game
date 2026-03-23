package notification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrMessageNotFound      = errors.New("message not found")
)

// NotificationRepository defines the interface for notification data operations
type NotificationRepository interface {
	CreateNotification(notif *Notification) error
	GetNotificationByID(id uint) (*Notification, error)
	ListNotifications(userID uint, offset, limit int) ([]*Notification, int64, error)
	ListUnreadNotifications(userID uint, offset, limit int) ([]*Notification, int64, error)
	MarkAsRead(id uint) error
	MarkAllAsRead(userID uint) error
	DeleteNotification(id uint) error
	GetUnreadCount(userID uint) (int64, error)
}

// MessageRepository defines the interface for message data operations
type MessageRepository interface {
	CreateMessage(msg *Message) error
	GetMessageByID(id uint) (*Message, error)
	ListMessages(userID uint, offset, limit int) ([]*Message, int64, error)
	DeleteMessage(id uint) error
}

// Service provides notification business logic
type Service struct {
	notifRepo NotificationRepository
	msgRepo   MessageRepository
}

// NewService creates a new notification service
func NewService(notifRepo NotificationRepository, msgRepo MessageRepository) *Service {
	return &Service{
		notifRepo: notifRepo,
		msgRepo:   msgRepo,
	}
}

// Notification operations

// CreateNotification creates a new notification
func (s *Service) CreateNotification(ctx context.Context, req *CreateNotificationRequest) (*Notification, error) {
	notif := &Notification{
		UserID:  req.UserID,
		Type:    req.Type,
		Title:   req.Title,
		Content: req.Content,
		Read:    false,
	}

	if err := s.notifRepo.CreateNotification(notif); err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	return notif, nil
}

// BroadcastNotification sends a notification to multiple users
func (s *Service) BroadcastNotification(ctx context.Context, userIDs []uint, req *BroadcastNotificationRequest) error {
	for _, userID := range userIDs {
		notif := &Notification{
			UserID:  userID,
			Type:    req.Type,
			Title:   req.Title,
			Content: req.Content,
			Read:    false,
		}
		if err := s.notifRepo.CreateNotification(notif); err != nil {
			// Log error but continue with other users
			fmt.Printf("failed to send notification to user %d: %v", userID, err)
		}
	}
	return nil
}

// GetNotification retrieves a notification by ID
func (s *Service) GetNotification(ctx context.Context, id uint) (*Notification, error) {
	return s.notifRepo.GetNotificationByID(id)
}

// ListNotifications lists notifications with pagination
func (s *Service) ListNotifications(ctx context.Context, userID uint, page, pageSize int) ([]*Notification, int64, error) {
	offset := (page - 1) * pageSize
	return s.notifRepo.ListNotifications(userID, offset, pageSize)
}

// ListUnreadNotifications lists unread notifications
func (s *Service) ListUnreadNotifications(ctx context.Context, userID uint, page, pageSize int) ([]*Notification, int64, error) {
	offset := (page - 1) * pageSize
	return s.notifRepo.ListUnreadNotifications(userID, offset, pageSize)
}

// MarkAsRead marks a notification as read
func (s *Service) MarkAsRead(ctx context.Context, id uint) error {
	return s.notifRepo.MarkAsRead(id)
}

// MarkAllAsRead marks all notifications as read for a user
func (s *Service) MarkAllAsRead(ctx context.Context, userID uint) error {
	return s.notifRepo.MarkAllAsRead(userID)
}

// DeleteNotification deletes a notification
func (s *Service) DeleteNotification(ctx context.Context, id uint) error {
	return s.notifRepo.DeleteNotification(id)
}

// GetUnreadCount retrieves the count of unread notifications
func (s *Service) GetUnreadCount(ctx context.Context, userID uint) (int64, error) {
	return s.notifRepo.GetUnreadCount(userID)
}

// Message operations

// CreateMessage creates a new message
func (s *Service) CreateMessage(ctx context.Context, req *CreateMessageRequest) (*Message, error) {
	msg := &Message{
		UserID:  req.UserID,
		Type:    req.Type,
		Title:   req.Title,
		Content: req.Content,
	}

	if err := s.msgRepo.CreateMessage(msg); err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	return msg, nil
}

// GetMessage retrieves a message by ID
func (s *Service) GetMessage(ctx context.Context, id uint) (*Message, error) {
	return s.msgRepo.GetMessageByID(id)
}

// ListMessages lists messages with pagination
func (s *Service) ListMessages(ctx context.Context, userID uint, page, pageSize int) ([]*Message, int64, error) {
	offset := (page - 1) * pageSize
	return s.msgRepo.ListMessages(userID, offset, pageSize)
}

// DeleteMessage deletes a message
func (s *Service) DeleteMessage(ctx context.Context, id uint) error {
	return s.msgRepo.DeleteMessage(id)
}

// Request types

// CreateNotificationRequest represents the request to create a notification
type CreateNotificationRequest struct {
	UserID  uint   `json:"user_id" binding:"required"`
	Type    string `json:"type" binding:"required,oneof=system game payment social"`
	Title   string `json:"title" binding:"required,min=1,max=100"`
	Content string `json:"content" binding:"required,max=1000"`
}

// BroadcastNotificationRequest represents the request to broadcast a notification
type BroadcastNotificationRequest struct {
	Type    string `json:"type" binding:"required,oneof=system game payment social"`
	Title   string `json:"title" binding:"required,min=1,max=100"`
	Content string `json:"content" binding:"required,max=1000"`
}

// CreateMessageRequest represents the request to create a message
type CreateMessageRequest struct {
	UserID  uint   `json:"user_id" binding:"required"`
	Type    string `json:"type" binding:"required,oneof=system game payment social"`
	Title   string `json:"title" binding:"required,min=1,max=100"`
	Content string `json:"content" binding:"required,max=5000"`
}

// Repository implementations

// NotificationRepositoryImpl implements NotificationRepository
type NotificationRepositoryImpl struct {
	db *gorm.DB
}

// NewNotificationRepositoryImpl creates a new notification repository
func NewNotificationRepositoryImpl(db *gorm.DB) NotificationRepository {
	return &NotificationRepositoryImpl{db: db}
}

func (r *NotificationRepositoryImpl) CreateNotification(notif *Notification) error {
	return r.db.Create(notif).Error
}

func (r *NotificationRepositoryImpl) GetNotificationByID(id uint) (*Notification, error) {
	var notif Notification
	err := r.db.First(&notif, id).Error
	if err != nil {
		return nil, err
	}
	return &notif, nil
}

func (r *NotificationRepositoryImpl) ListNotifications(userID uint, offset, limit int) ([]*Notification, int64, error) {
	var notifs []*Notification
	var total int64

	query := r.db.Model(&Notification{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&notifs).Error
	return notifs, total, err
}

func (r *NotificationRepositoryImpl) ListUnreadNotifications(userID uint, offset, limit int) ([]*Notification, int64, error) {
	var notifs []*Notification
	var total int64

	query := r.db.Model(&Notification{}).Where("user_id = ? AND read = ?", userID, false)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&notifs).Error
	return notifs, total, err
}

func (r *NotificationRepositoryImpl) MarkAsRead(id uint) error {
	return r.db.Model(&Notification{}).Where("id = ?", id).Update("read", true).Error
}

func (r *NotificationRepositoryImpl) MarkAllAsRead(userID uint) error {
	return r.db.Model(&Notification{}).Where("user_id = ?", userID).Update("read", true).Error
}

func (r *NotificationRepositoryImpl) DeleteNotification(id uint) error {
	return r.db.Delete(&Notification{}, id).Error
}

func (r *NotificationRepositoryImpl) GetUnreadCount(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&Notification{}).Where("user_id = ? AND read = ?", userID, false).Count(&count).Error
	return count, err
}

// MessageRepositoryImpl implements MessageRepository
type MessageRepositoryImpl struct {
	db *gorm.DB
}

// NewMessageRepositoryImpl creates a new message repository
func NewMessageRepositoryImpl(db *gorm.DB) MessageRepository {
	return &MessageRepositoryImpl{db: db}
}

func (r *MessageRepositoryImpl) CreateMessage(msg *Message) error {
	return r.db.Create(msg).Error
}

func (r *MessageRepositoryImpl) GetMessageByID(id uint) (*Message, error) {
	var msg Message
	err := r.db.First(&msg, id).Error
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *MessageRepositoryImpl) ListMessages(userID uint, offset, limit int) ([]*Message, int64, error) {
	var msgs []*Message
	var total int64

	query := r.db.Model(&Message{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&msgs).Error
	return msgs, total, err
}

func (r *MessageRepositoryImpl) DeleteMessage(id uint) error {
	return r.db.Delete(&Message{}, id).Error
}
