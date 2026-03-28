package game

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"online-game/pkg/api"
	"online-game/pkg/apperror"
)

// Handler handles HTTP requests for game service.
type Handler struct {
	service *Service
}

// NewHandler creates a new game handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers game routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	games := rg.Group("/games")
	{
		games.GET("", h.ListGames)
		games.GET("/:id", h.GetGame)
		games.GET("/:id/versions/latest", h.GetLatestVersion)
	}

	rooms := rg.Group("/rooms")
	{
		rooms.POST("", h.CreateRoom)
		rooms.GET("", h.ListRooms)
		rooms.GET("/:roomId", h.GetRoom)
		rooms.POST("/:roomId/join", h.JoinRoom)
		rooms.POST("/:roomId/leave", h.LeaveRoom)
		rooms.POST("/:roomId/start", h.StartGame)
		rooms.POST("/:roomId/action", h.PlayerAction)
	}
}

// ListGames handles GET /games
func (h *Handler) ListGames(c *gin.Context) {
	page, pageSize := api.GetPagination(c)
	query := &GameListQuery{
		GameType: c.Query("game_type"),
		Status:   c.Query("status"),
		Search:   c.Query("search"),
	}

	games, total, err := h.service.ListGames(query, page, pageSize)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Paginated(c, games, total, page, pageSize)
}

// GetGame handles GET /games/:id
func (h *Handler) GetGame(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.Error(c, apperror.ErrBadRequest.WithMessage("无效的游戏ID"))
		return
	}

	game, err := h.service.GetGame(uint(id))
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Success(c, game)
}

// GetLatestVersion handles GET /games/:id/versions/latest
func (h *Handler) GetLatestVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.Error(c, apperror.ErrBadRequest)
		return
	}

	version, err := h.service.GetLatestVersion(uint(id))
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Success(c, version)
}

// CreateRoom handles POST /rooms
func (h *Handler) CreateRoom(c *gin.Context) {
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.Error(c, apperror.ErrBadRequest.WithData(err.Error()))
		return
	}

	playerID := c.GetString("user_id")
	nickname := c.GetString("username")

	room, err := h.service.CreateRoom(c.Request.Context(), playerID, nickname, &req)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Created(c, room)
}

// ListRooms handles GET /rooms
func (h *Handler) ListRooms(c *gin.Context) {
	page, pageSize := api.GetPagination(c)
	gameID, _ := strconv.ParseUint(c.Query("game_id"), 10, 32)

	rooms, total, err := h.service.GetRooms(uint(gameID), page, pageSize)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Paginated(c, rooms, total, page, pageSize)
}

// GetRoom handles GET /rooms/:roomId
func (h *Handler) GetRoom(c *gin.Context) {
	roomID := c.Param("roomId")
	var room GameRoom
	if err := h.service.db.Where("room_id = ?", roomID).First(&room).Error; err != nil {
		api.Error(c, apperror.ErrRoomNotFound)
		return
	}
	api.Success(c, room)
}

// JoinRoom handles POST /rooms/:roomId/join
func (h *Handler) JoinRoom(c *gin.Context) {
	roomID := c.Param("roomId")
	playerID := c.GetString("user_id")
	nickname := c.GetString("username")

	if err := h.service.JoinRoom(roomID, playerID, nickname); err != nil {
		api.Error(c, err)
		return
	}
	api.SuccessWithMessage(c, "加入房间成功", nil)
}

// LeaveRoom handles POST /rooms/:roomId/leave
func (h *Handler) LeaveRoom(c *gin.Context) {
	roomID := c.Param("roomId")
	playerID := c.GetString("user_id")

	if err := h.service.LeaveRoom(roomID, playerID); err != nil {
		api.Error(c, err)
		return
	}
	api.SuccessWithMessage(c, "离开房间成功", nil)
}

// StartGame handles POST /rooms/:roomId/start
func (h *Handler) StartGame(c *gin.Context) {
	roomID := c.Param("roomId")

	if err := h.service.StartGame(roomID); err != nil {
		api.Error(c, err)
		return
	}
	api.SuccessWithMessage(c, "游戏开始", nil)
}

// PlayerAction handles POST /rooms/:roomId/action
func (h *Handler) PlayerAction(c *gin.Context) {
	roomID := c.Param("roomId")
	playerID := c.GetString("user_id")

	var body struct {
		Action string `json:"action" binding:"required"`
		Data   any    `json:"data"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		api.Error(c, apperror.ErrBadRequest.WithData(err.Error()))
		return
	}

	if err := h.service.HandlePlayerAction(roomID, playerID, body.Action, body.Data); err != nil {
		api.Error(c, err)
		return
	}
	api.Success(c, nil)
}
