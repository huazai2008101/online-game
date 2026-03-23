package id

import (
	"github.com/gin-gonic/gin"
	"online-game/pkg/api"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/id", h.GenerateID)
	r.POST("/id/batch", h.BatchGenerateID)
}

func (h *Handler) GenerateID(c *gin.Context) {
	InitGenerator(1)
	id := GenerateID()
	api.Success(c, gin.H{"id": id})
}

func (h *Handler) BatchGenerateID(c *gin.Context) {
	type BatchRequest struct {
		Count int `json:"count" binding:"min=1,max=1000"`
	}
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	InitGenerator(1)
	ids := BatchGenerate(req.Count)
	api.Success(c, gin.H{"ids": ids})
}
