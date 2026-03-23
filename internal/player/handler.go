package player

import (
	"gorm.io/gorm"
	"github.com/gin-gonic/gin"
	"online-game/pkg/api"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db}
}

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	players := r.Group("/players")
	{
		players.GET("", h.ListPlayers)
		players.GET("/:id", h.GetPlayer)
		players.PUT("/:id", h.UpdatePlayer)
		players.GET("/:id/stats", h.GetStats)
	}
}

func (h *Handler) ListPlayers(c *gin.Context) {
	api.Success(c, gin.H{"message": "ListPlayers"})
}

func (h *Handler) GetPlayer(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetPlayer"})
}

func (h *Handler) UpdatePlayer(c *gin.Context) {
	api.Success(c, gin.H{"message": "UpdatePlayer"})
}

func (h *Handler) GetStats(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetStats"})
}
