package item

import (
	"github.com/gin-gonic/gin"
	"online-game/pkg/api"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/items", h.ListItems)
	r.GET("/items/:id", h.GetItem)
	r.GET("/user/:user_id/items", h.GetUserItems)
}

func (h *Handler) ListItems(c *gin.Context) {
	api.Success(c, gin.H{"message": "ListItems"})
}

func (h *Handler) GetItem(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetItem"})
}

func (h *Handler) GetUserItems(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetUserItems"})
}
