package player

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"online-game/pkg/api"
)

// Handler handles HTTP requests for the player service
type Handler struct {
	service *Service
}

// NewHandler creates a new handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all routes for the player service
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	players := r.Group("/players")
	{
		players.POST("", h.CreatePlayer)
		players.GET("", h.ListPlayers)
		players.GET("/:id", h.GetPlayer)
		players.PUT("/:id", h.UpdatePlayer)
		players.DELETE("/:id", h.DeletePlayer)
		players.GET("/:id/stats", h.GetStats)
		players.POST("/:id/exp", h.AddExperience)
		players.POST("/:id/score", h.AddScore)
	}

	userGames := r.Group("/users/:user_id/games")
	{
		userGames.GET("", h.GetUserPlayers)
		userGames.GET("/:game_id", h.GetPlayerByUserAndGame)
	}
}

// CreatePlayer creates a new player
func (h *Handler) CreatePlayer(c *gin.Context) {
	var req CreatePlayerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	player, err := h.service.CreatePlayer(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "角色创建成功", player)
}

// ListPlayers lists players with pagination
func (h *Handler) ListPlayers(c *gin.Context) {
	var params api.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		params = api.DefaultPagination()
	}

	gameID, _ := strconv.ParseUint(c.Query("game_id"), 10, 32)

	players, total, err := h.service.ListPlayers(c.Request.Context(), uint(gameID), params.Page, params.PerPage)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Paginated(c, players, params.Page, params.PerPage, total)
}

// GetPlayer retrieves a player by ID
func (h *Handler) GetPlayer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的角色ID")
		return
	}

	player, err := h.service.GetPlayer(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, player)
}

// UpdatePlayerRequest represents the request to update a player
type UpdatePlayerRequest struct {
	Nickname *string `json:"nickname" binding:"omitempty,min=1,max=50"`
	Level    *int    `json:"level" binding:"omitempty,min=1,max=1000"`
	Exp      *int64  `json:"exp" binding:"omitempty,min=0"`
	Score    *int64  `json:"score" binding:"omitempty"`
	Status   *int    `json:"status" binding:"omitempty,min=0,max=2"`
}

// UpdatePlayer updates a player
func (h *Handler) UpdatePlayer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的角色ID")
		return
	}

	var req UpdatePlayerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if err := h.service.UpdatePlayer(c.Request.Context(), uint(id), &req); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "角色更新成功", nil)
}

// DeletePlayer deletes a player
func (h *Handler) DeletePlayer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的角色ID")
		return
	}

	if err := h.service.DeletePlayer(c.Request.Context(), uint(id)); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "角色删除成功", nil)
}

// GetStats retrieves player statistics
func (h *Handler) GetStats(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的角色ID")
		return
	}

	stats, err := h.service.GetStats(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, stats)
}

// AddExperienceRequest represents the request to add experience
type AddExperienceRequest struct {
	Exp int64 `json:"exp" binding:"required,min=1"`
}

// AddExperience adds experience to a player
func (h *Handler) AddExperience(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的角色ID")
		return
	}

	var req AddExperienceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	player, err := h.service.AddExperience(c.Request.Context(), uint(id), req.Exp)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "经验增加成功", player)
}

// AddScoreRequest represents the request to add score
type AddScoreRequest struct {
	Score int64 `json:"score" binding:"required"`
}

// AddScore adds score to a player
func (h *Handler) AddScore(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的角色ID")
		return
	}

	var req AddScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	player, err := h.service.AddScore(c.Request.Context(), uint(id), req.Score)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "积分增加成功", player)
}

// GetUserPlayers retrieves all players for a user
func (h *Handler) GetUserPlayers(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	players, err := h.service.ListPlayersByUserID(c.Request.Context(), uint(userID))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, gin.H{
		"user_id": uint(userID),
		"players": players,
	})
}

// GetPlayerByUserAndGame retrieves a player by user and game
func (h *Handler) GetPlayerByUserAndGame(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	gameID, err := strconv.ParseUint(c.Param("game_id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	player, err := h.service.GetPlayerByUserAndGame(c.Request.Context(), uint(userID), uint(gameID))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, player)
}

// CreatePlayerRequest represents the request to create a player
type CreatePlayerRequest struct {
	UserID   uint   `json:"user_id" binding:"required"`
	GameID   uint   `json:"game_id" binding:"required"`
	Nickname string `json:"nickname" binding:"required,min=1,max=50"`
}
