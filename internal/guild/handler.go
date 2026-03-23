package guild

import (
	"github.com/gin-gonic/gin"
	"online-game/pkg/api"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/guilds", h.ListGuilds)
	r.GET("/guilds/:id", h.GetGuild)
	r.POST("/guilds", h.CreateGuild)
}

func (h *Handler) ListGuilds(c *gin.Context) {
	api.Success(c, gin.H{"message": "ListGuilds"})
}

func (h *Handler) GetGuild(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetGuild"})
}

func (h *Handler) CreateGuild(c *gin.Context) {
	api.Success(c, gin.H{"message": "CreateGuild"})
}
