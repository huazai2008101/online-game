package game

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"online-game/pkg/api"
)

// Handler handles HTTP requests for the game service
type Handler struct {
	service *Service
}

// NewHandler creates a new handler
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// RegisterRoutes registers all routes for the game service
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	games := r.Group("/games")
	{
		// Game CRUD
		games.POST("", h.CreateGame)
		games.GET("", h.ListGames)
		games.GET("/:id", h.GetGame)
		games.PUT("/:id", h.UpdateGame)
		games.DELETE("/:id", h.DeleteGame)

		// Game config
		games.PUT("/:id/config", h.UpdateGameConfig)
		games.GET("/:id/config", h.GetGameConfig)

		// Game versions
		games.POST("/:id/versions", h.CreateGameVersion)
		games.GET("/:id/versions", h.ListGameVersions)

		// Game execution
		games.POST("/:id/start", h.StartGame)
		games.POST("/:id/stop", h.StopGame)
		games.GET("/:id/status", h.GetGameStatus)
		games.POST("/:id/action", h.PlayerAction)
	}

	rooms := r.Group("/rooms")
	{
		rooms.POST("", h.CreateRoom)
		rooms.GET("", h.ListRooms)
		rooms.GET("/:id", h.GetRoom)
		rooms.PUT("/:id", h.UpdateRoom)
		rooms.DELETE("/:id", h.CloseRoom)
		rooms.POST("/:id/join", h.JoinRoom)
		rooms.POST("/:id/leave", h.LeaveRoom)
	}

	sessions := r.Group("/sessions")
	{
		sessions.GET("/:id", h.GetSession)
		sessions.GET("/:id/state", h.GetSessionState)
	}
}

// CreateGameRequest represents the request to create a game
type CreateGameRequest struct {
	OrgID       int64  `json:"org_id" binding:"required"`
	GameCode    string `json:"game_code" binding:"required,min=1,max=50"`
	GameName    string `json:"game_name" binding:"required,min=1,max=100"`
	GameType    string `json:"game_type" binding:"required,min=1,max=20"`
	Description string `json:"description" binding:"max=500"`
}

// UpdateGameRequest represents the request to update a game
type UpdateGameRequest struct {
	GameName  string `json:"game_name" binding:"omitempty,min=1,max=100"`
	GameType  string `json:"game_type" binding:"omitempty,min=1,max=20"`
	GameIcon  string `json:"game_icon" binding:"omitempty,max=255"`
	GameCover string `json:"game_cover" binding:"omitempty,max=255"`
	Status    *int   `json:"status" binding:"omitempty,min=0,max=2"`
}

// CreateGame creates a new game
func (h *Handler) CreateGame(c *gin.Context) {
	var req CreateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	game, err := h.service.CreateGame(c.Request.Context(), &CreateGameRequest{
		OrgID:       req.OrgID,
		GameCode:    req.GameCode,
		GameName:    req.GameName,
		GameType:    req.GameType,
		Description: req.Description,
	})
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "游戏创建成功", game)
}

// ListGames lists games with pagination
func (h *Handler) ListGames(c *gin.Context) {
	var params api.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		params = api.DefaultPagination()
	}

	orgID, _ := strconv.ParseInt(c.Query("org_id"), 10, 64)

	games, total, err := h.service.ListGames(c.Request.Context(), orgID, params.Page, params.PerPage)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Paginated(c, games, params.Page, params.PerPage, total)
}

// GetGame retrieves a game by ID
func (h *Handler) GetGame(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	game, err := h.service.GetGame(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, game)
}

// UpdateGame updates a game
func (h *Handler) UpdateGame(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	var req UpdateGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if err := h.service.UpdateGame(c.Request.Context(), uint(id), &req); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "游戏更新成功", nil)
}

// DeleteGame deletes a game
func (h *Handler) DeleteGame(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	if err := h.service.DeleteGame(c.Request.Context(), uint(id)); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "游戏删除成功", nil)
}

// UpdateGameConfigRequest represents the request to update game config
type UpdateGameConfigRequest struct {
	Config map[string]interface{} `json:"config"`
}

// UpdateGameConfig updates game configuration
func (h *Handler) UpdateGameConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	var req UpdateGameConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if err := h.service.UpdateGameConfig(c.Request.Context(), uint(id), req.Config); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "配置更新成功", nil)
}

// GetGameConfig retrieves game configuration
func (h *Handler) GetGameConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	config, err := h.service.GetGameConfig(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, config)
}

// CreateGameVersionRequest represents the request to create a game version
type CreateGameVersionRequest struct {
	Version     string `json:"version" binding:"required,min=1,max=20"`
	ScriptType  string `json:"script_type" binding:"required,oneof=js wasm"`
	ScriptPath  string `json:"script_path" binding:"required,max=255"`
	ScriptHash  string `json:"script_hash" binding:"required,max=64"`
	Description string `json:"description" binding:"max=500"`
}

// CreateGameVersion creates a new game version
func (h *Handler) CreateGameVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	var req CreateGameVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	// Use gameID from path parameter
	req.GameID = uint(id)

	version, err := h.service.CreateGameVersion(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "版本创建成功", version)
}

// ListGameVersions lists game versions
func (h *Handler) ListGameVersions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	versions, err := h.service.ListGameVersions(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, gin.H{"versions": versions})
}

// StartGameRequest represents the request to start a game
type StartGameRequest struct {
	RoomID  string                 `json:"room_id" binding:"required"`
	Players []string               `json:"players" binding:"required,min=1"`
	Config  map[string]interface{} `json:"config"`
}

// StartGame starts a game instance
func (h *Handler) StartGame(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	var req StartGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	session, err := h.service.StartGame(c.Request.Context(), uint(id), &StartGameRequest{
		RoomID:  req.RoomID,
		Players: req.Players,
		Config:  req.Config,
	})
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "游戏已启动", gin.H{
		"room_id":    req.RoomID,
		"session_id": session.ID,
		"status":     session.Status,
	})
}

// StopGameRequest represents the request to stop a game
type StopGameRequest struct {
	RoomID string `json:"room_id" binding:"required"`
	Reason string `json:"reason"`
}

// StopGame stops a running game
func (h *Handler) StopGame(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	var req StopGameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if err := h.service.StopGame(c.Request.Context(), uint(id), req.RoomID, req.Reason); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "游戏已停止", nil)
}

// GetGameStatus retrieves the status of a running game
func (h *Handler) GetGameStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	roomID := c.Query("room_id")
	if roomID == "" {
		api.BadRequest(c, "缺少room_id参数")
		return
	}

	status, err := h.service.GetGameStatus(c.Request.Context(), uint(id), roomID)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, status)
}

// PlayerActionRequest represents a player action
type PlayerActionRequest struct {
	RoomID   string                 `json:"room_id" binding:"required"`
	PlayerID string                 `json:"player_id" binding:"required"`
	Action   string                 `json:"action" binding:"required"`
	Data     map[string]interface{} `json:"data"`
}

// PlayerAction sends a player action to the game
func (h *Handler) PlayerAction(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	var req PlayerActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if err := h.service.PlayerAction(c.Request.Context(), uint(id), req.RoomID, req.PlayerID, req.Action, req.Data); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "操作已执行", nil)
}

// CreateRoomRequest represents the request to create a room
type CreateRoomRequest struct {
	RoomID     string                 `json:"room_id" binding:"required,min=1,max=50"`
	GameID     uint                   `json:"game_id" binding:"required"`
	RoomName   string                 `json:"room_name" binding:"required,min=1,max=100"`
	MaxPlayers int                    `json:"max_players" binding:"min=2,max=100"`
	Config     map[string]interface{} `json:"config"`
}

// CreateRoom creates a new game room
func (h *Handler) CreateRoom(c *gin.Context) {
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	room, err := h.service.CreateRoom(c.Request.Context(), &CreateRoomRequest{
		RoomID:     req.RoomID,
		GameID:     req.GameID,
		RoomName:   req.RoomName,
		MaxPlayers: req.MaxPlayers,
		Config:     req.Config,
	})
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "房间创建成功", room)
}

// ListRooms lists rooms
func (h *Handler) ListRooms(c *gin.Context) {
	var params api.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		params = api.DefaultPagination()
	}

	gameID, _ := strconv.ParseUint(c.Query("game_id"), 10, 32)
	status := c.Query("status")

	rooms, total, err := h.service.ListRooms(c.Request.Context(), uint(gameID), status, params.Page, params.PerPage)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Paginated(c, rooms, params.Page, params.PerPage, total)
}

// GetRoom retrieves a room by ID
func (h *Handler) GetRoom(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的房间ID")
		return
	}

	room, err := h.service.GetRoom(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, room)
}

// UpdateRoomRequest represents the request to update a room
type UpdateRoomRequest struct {
	Status     *int                    `json:"status"`
	MaxPlayers *int                    `json:"max_players" binding:"omitempty,min=2,max=100"`
	Config     map[string]interface{}  `json:"config"`
}

// UpdateRoom updates a room
func (h *Handler) UpdateRoom(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的房间ID")
		return
	}

	var req UpdateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	updates := make(map[string]interface{})
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.MaxPlayers != nil {
		updates["max_players"] = *req.MaxPlayers
	}
	if req.Config != nil {
		updates["config"] = req.Config
	}

	if err := h.service.UpdateRoom(c.Request.Context(), uint(id), updates); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "房间更新成功", nil)
}

// CloseRoom closes a room
func (h *Handler) CloseRoom(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的房间ID")
		return
	}

	if err := h.service.CloseRoom(c.Request.Context(), uint(id)); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "房间已关闭", nil)
}

// JoinRoomRequest represents the request to join a room
type JoinRoomRequest struct {
	PlayerID string `json:"player_id" binding:"required"`
}

// JoinRoom adds a player to a room
func (h *Handler) JoinRoom(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的房间ID")
		return
	}

	var req JoinRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	// Get room ID from the room itself
	room, err := h.service.GetRoom(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	if err := h.service.JoinRoom(c.Request.Context(), room.RoomID, req.PlayerID); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "已加入房间", nil)
}

// LeaveRoomRequest represents the request to leave a room
type LeaveRoomRequest struct {
	PlayerID string `json:"player_id" binding:"required"`
}

// LeaveRoom removes a player from a room
func (h *Handler) LeaveRoom(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的房间ID")
		return
	}

	var req LeaveRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	// Get room ID from the room itself
	room, err := h.service.GetRoom(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	if err := h.service.LeaveRoom(c.Request.Context(), room.RoomID, req.PlayerID); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "已离开房间", nil)
}

// GetSession retrieves a session by ID
func (h *Handler) GetSession(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的会话ID")
		return
	}

	session, err := h.service.GetSession(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, session)
}

// GetSessionState retrieves the state of a session
func (h *Handler) GetSessionState(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的会话ID")
		return
	}

	state, err := h.service.GetSessionState(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, state)
}
