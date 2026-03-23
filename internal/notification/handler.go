package notification

import (
	"github.com/gin-gonic/gin"
	"online-game/pkg/api"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/notifications", h.ListNotifications)
	r.POST("/notifications/send", h.SendNotification)
	r.GET("/messages", h.ListMessages)
}

func (h *Handler) ListNotifications(c *gin.Context) {
	api.Success(c, gin.H{"message": "ListNotifications"})
}

func (h *Handler) SendNotification(c *gin.Context) {
	api.Success(c, gin.H{"message": "SendNotification"})
}

func (h *Handler) ListMessages(c *gin.Context) {
	api.Success(c, gin.H{"message": "ListMessages"})
}
