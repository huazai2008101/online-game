package activity

import (
	"github.com/gin-gonic/gin"
	"online-game/pkg/api"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/activities", h.ListActivities)
	r.GET("/activities/:id", h.GetActivity)
}

func (h *Handler) ListActivities(c *gin.Context) {
	api.Success(c, gin.H{"message": "ListActivities"})
}

func (h *Handler) GetActivity(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetActivity"})
}
