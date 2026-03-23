package file

import (
	"github.com/gin-gonic/gin"
	"online-game/pkg/api"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/upload", h.Upload)
	r.GET("/files/:id", h.GetFile)
	r.DELETE("/files/:id", h.DeleteFile)
}

func (h *Handler) Upload(c *gin.Context) {
	api.Success(c, gin.H{"message": "Upload"})
}

func (h *Handler) GetFile(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetFile"})
}

func (h *Handler) DeleteFile(c *gin.Context) {
	api.Success(c, gin.H{"message": "DeleteFile"})
}
