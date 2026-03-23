package organization

import (
	"github.com/gin-gonic/gin"
	"online-game/pkg/api"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/orgs", h.ListOrgs)
	r.POST("/orgs", h.CreateOrg)
	r.GET("/orgs/:id", h.GetOrg)
	r.GET("/orgs/:id/members", h.GetMembers)
}

func (h *Handler) ListOrgs(c *gin.Context) {
	api.Success(c, gin.H{"message": "ListOrgs"})
}

func (h *Handler) CreateOrg(c *gin.Context) {
	api.Success(c, gin.H{"message": "CreateOrg"})
}

func (h *Handler) GetOrg(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetOrg"})
}

func (h *Handler) GetMembers(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetMembers"})
}
