package game

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"online-game/pkg/actor"
	"online-game/pkg/api"
)

// Handler handles HTTP requests for the game service
type Handler struct {
	repo *Repository
	mgr  *GameInstanceManager
}

// NewHandler creates a new handler
func NewHandler(repo *Repository, mgr *GameInstanceManager) *Handler {
	return &Handler{
		repo: repo,
		mgr:  mgr,
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

	game := &Game{
		OrgID:    req.OrgID,
		GameCode: req.GameCode,
		GameName: req.GameName,
		GameType: req.GameType,
		Status:   1,
	}

	if err := h.repo.CreateGame(game); err != nil {
		api.InternalError(c, "创建游戏失败")
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

	games, total, err := h.repo.ListGames(orgID, params.GetOffset(), params.PerPage)
	if err != nil {
		api.InternalError(c, "获取游戏列表失败")
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

	game, err := h.repo.GetGameByID(uint(id))
	if err != nil {
		api.NotFound(c, "游戏不存在")
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

	game, err := h.repo.GetGameByID(uint(id))
	if err != nil {
		api.NotFound(c, "游戏不存在")
		return
	}

	if req.GameName != "" {
		game.GameName = req.GameName
	}
	if req.GameType != "" {
		game.GameType = req.GameType
	}
	if req.GameIcon != "" {
		game.GameIcon = req.GameIcon
	}
	if req.GameCover != "" {
		game.GameCover = req.GameCover
	}
	if req.Status != nil {
		game.Status = *req.Status
	}

	if err := h.repo.UpdateGame(game); err != nil {
		api.InternalError(c, "更新游戏失败")
		return
	}

	api.SuccessWithMessage(c, "游戏更新成功", game)
}

// DeleteGame deletes a game
func (h *Handler) DeleteGame(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	if err := h.repo.DeleteGame(uint(id)); err != nil {
		api.InternalError(c, "删除游戏失败")
		return
	}

	api.SuccessWithMessage(c, "游戏删除成功", nil)
}

// UpdateGameConfig updates game configuration
func (h *Handler) UpdateGameConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	var config map[string]interface{}
	if err := c.ShouldBindJSON(&config); err != nil {
		api.ValidationError(c, err)
		return
	}

	game, err := h.repo.GetGameByID(uint(id))
	if err != nil {
		api.NotFound(c, "游戏不存在")
		return
	}

	// In a real implementation, we'd serialize the config to JSON
	// For now, just store it as a string
	game.GameConfig = "config_updated"

	if err := h.repo.UpdateGame(game); err != nil {
		api.InternalError(c, "更新配置失败")
		return
	}

	api.SuccessWithMessage(c, "配置更新成功", game)
}

// GetGameConfig retrieves game configuration
func (h *Handler) GetGameConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的游戏ID")
		return
	}

	game, err := h.repo.GetGameByID(uint(id))
	if err != nil {
		api.NotFound(c, "游戏不存在")
		return
	}

	// Return the config
	api.Success(c, gin.H{
		"game_id": game.ID,
		"config":   game.GameConfig,
	})
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

	version := &GameVersion{
		GameID:      uint(id),
		Version:     req.Version,
		ScriptType:  req.ScriptType,
		ScriptPath:  req.ScriptPath,
		ScriptHash:  req.ScriptHash,
		Description: req.Description,
	}

	if err := h.repo.db.Create(version).Error; err != nil {
		api.InternalError(c, "创建版本失败")
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

	var versions []*GameVersion
	err = h.repo.db.Where("game_id = ?", uint(id)).
		Order("created_at DESC").
		Find(&versions).Error

	if err != nil {
		api.InternalError(c, "获取版本列表失败")
		return
	}

	api.Success(c, versions)
}

// StartGameRequest represents the request to start a game
type StartGameRequest struct {
	RoomID   string        `json:"room_id" binding:"required"`
	Players  []string      `json:"players" binding:"required,min=1"`
	Config   map[string]any `json:"config"`
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

	gameID := strconv.FormatUint(id, 10)

	// Create game actor
	gameActor, err := h.mgr.CreateGameActor(gameID, req.RoomID)
	if err != nil {
		api.InternalError(c, "创建游戏实例失败")
		return
	}

	// Create engine
	engine, err := h.mgr.GetGameEngine(gameID)
	if err != nil {
		api.InternalError(c, "创建游戏引擎失败")
		return
	}

	// Send start message
	gameActor.Send(&actor.GameStartMessage{
		GameID:    gameID,
		RoomID:    req.RoomID,
		Players:   req.Players,
		Timestamp: 0,
	})

	// Create session
	session := &GameSession{
		RoomID:    req.RoomID,
		GameID:    uint(id),
		Status:    "running",
	}

	_ = h.repo.CreateSession(session)

	api.SuccessWithMessage(c, "游戏已启动", gin.H{
		"room_id":   req.RoomID,
		"session_id": session.ID,
		"engine":    engine.CurrentEngine(),
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

	gameID := strconv.FormatUint(id, 10)

	// Stop game actor
	if err := h.mgr.StopGameActor(gameID, req.RoomID); err != nil {
		api.InternalError(c, "停止游戏失败")
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

	gameID := strconv.FormatUint(id, 10)

	actor, err := h.mgr.GetGameActor(gameID, roomID)
	if err != nil {
		api.NotFound(c, "游戏实例不存在")
		return
	}

	stats := actor.Stats()

	api.Success(c, gin.H{
		"game_id":         gameID,
		"room_id":         roomID,
		"is_running":      actor.IsRunning(),
		"player_count":    actor.GetPlayerCount(),
		"messages_processed": stats.MessageCount,
	})
}

// CreateRoomRequest represents the request to create a room
type CreateRoomRequest struct {
	RoomID    string        `json:"room_id" binding:"required,min=1,max=50"`
	GameID    uint          `json:"game_id" binding:"required"`
	RoomName  string        `json:"room_name" binding:"required,min=1,max=100"`
	MaxPlayers int          `json:"max_players" binding:"min=2,max=100"`
	Config    map[string]any `json:"config"`
}

// CreateRoom creates a new game room
func (h *Handler) CreateRoom(c *gin.Context) {
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	room := &GameRoom{
		RoomID:     req.RoomID,
		GameID:     req.GameID,
		RoomName:   req.RoomName,
		MaxPlayers: req.MaxPlayers,
		Status:     "waiting",
	}

	if err := h.repo.CreateRoom(room); err != nil {
		api.InternalError(c, "创建房间失败")
		return
	}

	// Create room actor
	roomActor := actor.NewRoomActor(req.RoomID, strconv.FormatUint(uint64(req.GameID), 10), req.MaxPlayers)
	h.mgr.actorSystem.Register(roomActor)
	roomActor.Start(c)

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

	rooms, total, err := h.repo.ListRooms(uint(gameID), status, params.GetOffset(), params.PerPage)
	if err != nil {
		api.InternalError(c, "获取房间列表失败")
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

	room, err := h.repo.GetRoomByID(uint(id))
	if err != nil {
		api.NotFound(c, "房间不存在")
		return
	}

	api.Success(c, room)
}

// UpdateRoom updates a room
func (h *Handler) UpdateRoom(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的房间ID")
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	room, err := h.repo.GetRoomByID(uint(id))
	if err != nil {
		api.NotFound(c, "房间不存在")
		return
	}

	if err := h.repo.UpdateRoom(room); err != nil {
		api.InternalError(c, "更新房间失败")
		return
	}

	api.SuccessWithMessage(c, "房间更新成功", room)
}

// CloseRoom closes a room
func (h *Handler) CloseRoom(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的房间ID")
		return
	}

	room, err := h.repo.GetRoomByID(uint(id))
	if err != nil {
		api.NotFound(c, "房间不存在")
		return
	}

	room.Status = "closed"
	if err := h.repo.UpdateRoom(room); err != nil {
		api.InternalError(c, "关闭房间失败")
		return
	}

	api.SuccessWithMessage(c, "房间已关闭", room)
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

	room, err := h.repo.GetRoomByID(uint(id))
	if err != nil {
		api.NotFound(c, "房间不存在")
		return
	}

	if room.PlayerCount >= room.MaxPlayers {
		api.BadRequest(c, "房间已满")
		return
	}

	room.PlayerCount++
	if err := h.repo.UpdateRoom(room); err != nil {
		api.InternalError(c, "加入房间失败")
		return
	}

	api.SuccessWithMessage(c, "已加入房间", room)
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

	room, err := h.repo.GetRoomByID(uint(id))
	if err != nil {
		api.NotFound(c, "房间不存在")
		return
	}

	if room.PlayerCount > 0 {
		room.PlayerCount--
		if err := h.repo.UpdateRoom(room); err != nil {
			api.InternalError(c, "离开房间失败")
			return
		}
	}

	api.SuccessWithMessage(c, "已离开房间", room)
}

// GetSession retrieves a session by ID
func (h *Handler) GetSession(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的会话ID")
		return
	}

	session, err := h.repo.GetSessionByID(uint(id))
	if err != nil {
		api.NotFound(c, "会话不存在")
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

	session, err := h.repo.GetSessionByID(uint(id))
	if err != nil {
		api.NotFound(c, "会话不存在")
		return
	}

	// Return the final state
	api.Success(c, gin.H{
		"session_id": session.ID,
		"status":     session.Status,
		"state":      session.FinalState,
	})
}
