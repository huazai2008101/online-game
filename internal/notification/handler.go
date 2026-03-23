package notification

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"online-game/pkg/api"
)

// Handler handles HTTP requests for the notification service
type Handler struct {
	service *Service
}

// NewHandler creates a new handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all routes for the notification service
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	// Notifications
	notifications := r.Group("/notifications")
	{
		notifications.POST("", h.CreateNotification)
		notifications.POST("/broadcast", h.BroadcastNotification)
		notifications.GET("", h.ListNotifications)
		notifications.GET("/unread", h.ListUnreadNotifications)
		notifications.GET("/count", h.GetUnreadCount)
		notifications.GET("/:id", h.GetNotification)
		notifications.PUT("/:id/read", h.MarkAsRead)
		notifications.PUT("/read-all", h.MarkAllAsRead)
		notifications.DELETE("/:id", h.DeleteNotification)
	}

	// Messages
	messages := r.Group("/messages")
	{
		messages.POST("", h.CreateMessage)
		messages.GET("", h.ListMessages)
		messages.GET("/:id", h.GetMessage)
		messages.DELETE("/:id", h.DeleteMessage)
	}
}

// CreateNotification creates a new notification
func (h *Handler) CreateNotification(c *gin.Context) {
	var req CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	notif, err := h.service.CreateNotification(c.Request.Context(), &req)
	if err != nil {
		api.InternalError(c, "创建通知失败")
		return
	}

	api.SuccessWithMessage(c, "通知创建成功", notif)
}

// BroadcastNotificationRequest represents the request to broadcast a notification
type BroadcastNotificationRequest struct {
	UserIDs []uint                   `json:"user_ids" binding:"required"`
	Type    string                   `json:"type" binding:"required,oneof=system game payment social"`
	Title   string                   `json:"title" binding:"required,min=1,max=100"`
	Content string                   `json:"content" binding:"required,max=1000"`
}

// BroadcastNotification broadcasts a notification to multiple users
func (h *Handler) BroadcastNotification(c *gin.Context) {
	var req BroadcastNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if err := h.service.BroadcastNotification(c.Request.Context(), req.UserIDs, &BroadcastNotificationRequest{
		Type:    req.Type,
		Title:   req.Title,
		Content: req.Content,
	}); err != nil {
		api.InternalError(c, "广播通知失败")
		return
	}

	api.SuccessWithMessage(c, "通知已发送", gin.H{
		"count": len(req.UserIDs),
	})
}

// ListNotifications lists notifications with pagination
func (h *Handler) ListNotifications(c *gin.Context) {
	var params api.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		params = api.DefaultPagination()
	}

	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 32)

	notifications, total, err := h.service.ListNotifications(c.Request.Context(), uint(userID), params.Page, params.PerPage)
	if err != nil {
		api.InternalError(c, "获取通知列表失败")
		return
	}

	api.Paginated(c, notifications, params.Page, params.PerPage, total)
}

// ListUnreadNotifications lists unread notifications
func (h *Handler) ListUnreadNotifications(c *gin.Context) {
	var params api.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		params = api.DefaultPagination()
	}

	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 32)

	notifications, total, err := h.service.ListUnreadNotifications(c.Request.Context(), uint(userID), params.Page, params.PerPage)
	if err != nil {
		api.InternalError(c, "获取未读通知失败")
		return
	}

	api.Paginated(c, notifications, params.Page, params.PerPage, total)
}

// GetUnreadCount retrieves the count of unread notifications
func (h *Handler) GetUnreadCount(c *gin.Context) {
	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 32)

	count, err := h.service.GetUnreadCount(c.Request.Context(), uint(userID))
	if err != nil {
		api.InternalError(c, "获取未读数量失败")
		return
	}

	api.Success(c, gin.H{
		"user_id":      uint(userID),
		"unread_count": count,
	})
}

// GetNotification retrieves a notification by ID
func (h *Handler) GetNotification(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的通知ID")
		return
	}

	notification, err := h.service.GetNotification(c.Request.Context(), uint(id))
	if err != nil {
		api.NotFound(c, "通知不存在")
		return
	}

	api.Success(c, notification)
}

// MarkAsRead marks a notification as read
func (h *Handler) MarkAsRead(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的通知ID")
		return
	}

	if err := h.service.MarkAsRead(c.Request.Context(), uint(id)); err != nil {
		api.InternalError(c, "标记失败")
		return
	}

	api.SuccessWithMessage(c, "已标记为已读", nil)
}

// MarkAllAsReadRequest represents the request to mark all as read
type MarkAllAsReadRequest struct {
	UserID uint `json:"user_id" binding:"required"`
}

// MarkAllAsRead marks all notifications as read for a user
func (h *Handler) MarkAllAsRead(c *gin.Context) {
	var req MarkAllAsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if err := h.service.MarkAllAsRead(c.Request.Context(), req.UserID); err != nil {
		api.InternalError(c, "标记失败")
		return
	}

	api.SuccessWithMessage(c, "已标记所有通知为已读", nil)
}

// DeleteNotification deletes a notification
func (h *Handler) DeleteNotification(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的通知ID")
		return
	}

	if err := h.service.DeleteNotification(c.Request.Context(), uint(id)); err != nil {
		api.InternalError(c, "删除通知失败")
		return
	}

	api.SuccessWithMessage(c, "通知已删除", nil)
}

// CreateMessage creates a new message
func (h *Handler) CreateMessage(c *gin.Context) {
	var req CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	msg, err := h.service.CreateMessage(c.Request.Context(), &req)
	if err != nil {
		api.InternalError(c, "创建消息失败")
		return
	}

	api.SuccessWithMessage(c, "消息创建成功", msg)
}

// ListMessages lists messages with pagination
func (h *Handler) ListMessages(c *gin.Context) {
	var params api.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		params = api.DefaultPagination()
	}

	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 32)

	messages, total, err := h.service.ListMessages(c.Request.Context(), uint(userID), params.Page, params.PerPage)
	if err != nil {
		api.InternalError(c, "获取消息列表失败")
		return
	}

	api.Paginated(c, messages, params.Page, params.PerPage, total)
}

// GetMessage retrieves a message by ID
func (h *Handler) GetMessage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的消息ID")
		return
	}

	message, err := h.service.GetMessage(c.Request.Context(), uint(id))
	if err != nil {
		api.NotFound(c, "消息不存在")
		return
	}

	api.Success(c, message)
}

// DeleteMessage deletes a message
func (h *Handler) DeleteMessage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的消息ID")
		return
	}

	if err := h.service.DeleteMessage(c.Request.Context(), uint(id)); err != nil {
		api.InternalError(c, "删除消息失败")
		return
	}

	api.SuccessWithMessage(c, "消息已删除", nil)
}
