package permission

import (
	"github.com/gin-gonic/gin"
	"online-game/pkg/api"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/roles", h.ListRoles)
	r.POST("/roles", h.CreateRole)
	r.GET("/permissions", h.ListPermissions)
	r.GET("/users/:user_id/roles", h.GetUserRoles)
}

func (h *Handler) ListRoles(c *gin.Context) {
	api.Success(c, gin.H{"message": "ListRoles"})
}

func (h *Handler) CreateRole(c *gin.Context) {
	api.Success(c, gin.H{"message": "CreateRole"})
}

func (h *Handler) ListPermissions(c *gin.Context) {
	api.Success(c, gin.H{"message": "ListPermissions"})
}

func (h *Handler) GetUserRoles(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetUserRoles"})
}
