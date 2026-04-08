package admin

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"online-game/pkg/api"
	"online-game/pkg/apperror"
	"online-game/internal/game"
)

// Service handles admin operations: game CRUD, package upload, publish.
type Service struct {
	db          *gorm.DB
	gameSvc     GameServiceClient
	storagePath string
}

// GameServiceClient abstracts calls to game-service (future gRPC).
type GameServiceClient interface {
	GetGame(id uint) (*game.Game, error)
}

// NewService creates a new admin service.
func NewService(db *gorm.DB, storagePath string) *Service {
	return &Service{db: db, storagePath: storagePath}
}

// --- Game CRUD ---

// CreateGame creates a new game entry.
func (s *Service) CreateGame(gameCode, gameName, gameType string, minPlayers, maxPlayers int) (*game.Game, error) {
	g := &game.Game{
		GameCode:   gameCode,
		GameName:   gameName,
		GameType:   gameType,
		MinPlayers: minPlayers,
		MaxPlayers: maxPlayers,
		Status:     "draft",
	}
	if err := s.db.Create(g).Error; err != nil {
		return nil, apperror.ErrDatabaseError.WithData(err.Error())
	}
	return g, nil
}

// UpdateGame updates game metadata.
func (s *Service) UpdateGame(id uint, updates map[string]any) (*game.Game, error) {
	if err := s.db.Model(&game.Game{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, apperror.ErrDatabaseError.WithData(err.Error())
	}
	return s.GetGame(id)
}

// GetGame retrieves a game by ID.
func (s *Service) GetGame(id uint) (*game.Game, error) {
	var g game.Game
	if err := s.db.First(&g, id).Error; err != nil {
		return nil, apperror.ErrGameNotFound
	}
	return &g, nil
}

// DeleteGame soft-deletes a game.
func (s *Service) DeleteGame(id uint) error {
	if err := s.db.Delete(&game.Game{}, id).Error; err != nil {
		return apperror.ErrDatabaseError.WithData(err.Error())
	}
	return nil
}

// ListGames lists all games (admin sees all including drafts).
func (s *Service) ListGames(page, pageSize int) ([]game.Game, int64, error) {
	var games []game.Game
	var total int64
	s.db.Model(&game.Game{}).Count(&total)
	offset := (page - 1) * pageSize
	if err := s.db.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&games).Error; err != nil {
		return nil, 0, apperror.ErrDatabaseError.WithData(err.Error())
	}
	return games, total, nil
}

// --- Package Upload ---

// Manifest defines the game package manifest format.
type Manifest struct {
	Name       string         `json:"name"`
	Version    string         `json:"version"`
	GameCode   string         `json:"gameCode"`
	GameType   string         `json:"gameType"`
	Entry      string         `json:"entry"`       // server/main.js
	ClientEntry string        `json:"clientEntry"` // client/index.html
	MinPlayers int            `json:"minPlayers"`
	MaxPlayers int            `json:"maxPlayers"`
	Config     map[string]any `json:"config"`
}

// UploadPackage handles game package upload (zip).
func (s *Service) UploadPackage(gameID uint, version string, zipData []byte) (*game.GameVersion, error) {
	// Verify game exists
	if _, err := s.GetGame(gameID); err != nil {
		return nil, err
	}

	// Create temp file
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("upload_%d_%d.zip", gameID, time.Now().UnixMilli()))
	if err := os.WriteFile(tmpPath, zipData, 0644); err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	defer os.Remove(tmpPath)

	// Extract and validate
	manifest, hash, size, err := s.extractAndValidate(tmpPath, gameID, version)
	if err != nil {
		return nil, err
	}

	// Save version record
	pkgPath := fmt.Sprintf("%s/games/%d/%s", s.storagePath, gameID, version)
	gv := &game.GameVersion{
		GameID:      gameID,
		Version:     version,
		ScriptType:  "js",
		PackagePath: pkgPath,
		PackageHash: hash,
		PackageSize: size,
		EntryScript: manifest.Entry,
		Status:      "active",
		CreatedBy:   0, // TODO: get from auth context
	}

	if err := s.db.Create(gv).Error; err != nil {
		return nil, apperror.ErrDatabaseError.WithData(err.Error())
	}

	slog.Info("game package uploaded",
		"game_id", gameID,
		"version", version,
		"size", size,
		"hash", hash,
	)
	return gv, nil
}

// PublishGame publishes a game version.
func (s *Service) PublishGame(gameID uint, version string) (*game.Game, error) {
	// Verify version exists
	var gv game.GameVersion
	if err := s.db.Where("game_id = ? AND version = ?", gameID, version).First(&gv).Error; err != nil {
		return nil, apperror.ErrNotFound.WithMessage("版本不存在")
	}

	// Update game status
	if err := s.db.Model(&game.Game{}).Where("id = ?", gameID).
		Updates(map[string]any{"status": "published"}).Error; err != nil {
		return nil, apperror.ErrDatabaseError.WithData(err.Error())
	}

	return s.GetGame(gameID)
}

// UnpublishGame takes a game offline.
func (s *Service) UnpublishGame(gameID uint) error {
	return s.db.Model(&game.Game{}).Where("id = ?", gameID).
		Update("status", "offline").Error
}

// Migrate runs auto migration.
func (s *Service) Migrate() error {
	return s.db.AutoMigrate(&game.Game{}, &game.GameVersion{})
}

// --- Internal ---

func (s *Service) extractAndValidate(zipPath string, gameID uint, version string) (*Manifest, string, int64, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, "", 0, apperror.ErrInvalidPackage.WithData(err.Error())
	}
	defer r.Close()

	// Find and parse manifest.json
	var manifestData []byte
	for _, f := range r.File {
		if filepath.Base(f.Name) == "manifest.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, "", 0, apperror.ErrInvalidPackage.WithData("无法读取manifest.json")
			}
			manifestData, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, "", 0, apperror.ErrInvalidPackage.WithData("manifest.json读取失败")
			}
			break
		}
	}
	if manifestData == nil {
		return nil, "", 0, apperror.ErrInvalidPackage.WithMessage("缺少manifest.json")
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, "", 0, apperror.ErrInvalidPackage.WithMessage("manifest.json格式错误")
	}

	// Validate required fields
	if manifest.Name == "" || manifest.Version == "" || manifest.Entry == "" {
		return nil, "", 0, apperror.ErrInvalidPackage.WithMessage("manifest缺少必要字段")
	}

	// Check entry script exists
	entryFound := false
	for _, f := range r.File {
		if f.Name == manifest.Entry {
			entryFound = true
			break
		}
	}
	if !entryFound {
		return nil, "", 0, apperror.ErrInvalidPackage.WithMessage("入口脚本不存在: " + manifest.Entry)
	}

	// Extract all files to storage
	destDir := filepath.Join(s.storagePath, "games", fmt.Sprintf("%d", gameID), version)
	os.MkdirAll(destDir, 0755)

	fileSize := int64(0)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		destFile := filepath.Join(destDir, f.Name)
		os.MkdirAll(filepath.Dir(destFile), 0755)

		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		fileSize += int64(len(data))
		os.WriteFile(destFile, data, 0644)
	}

	// Calculate hash from zip file
	stat, _ := os.Stat(zipPath)

	return &manifest, fmt.Sprintf("%x", stat.ModTime().UnixNano()), stat.Size(), nil
}

// --- Handler ---

// Handler handles admin HTTP requests.
type Handler struct {
	service *Service
}

// NewHandler creates a new admin handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers admin routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	admin := rg.Group("/admin")
	{
		admin.POST("/games", h.CreateGame)
		admin.GET("/games", h.ListGames)
		admin.GET("/games/:id", h.GetGame)
		admin.PUT("/games/:id", h.UpdateGame)
		admin.DELETE("/games/:id", h.DeleteGame)
		admin.POST("/games/:id/upload", h.UploadPackage)
		admin.POST("/games/:id/publish", h.PublishGame)
		admin.POST("/games/:id/unpublish", h.UnpublishGame)
	}
}

// CreateGame handles POST /admin/games
func (h *Handler) CreateGame(c *gin.Context) {
	var req struct {
		GameCode   string `json:"game_code" binding:"required"`
		GameName   string `json:"game_name" binding:"required"`
		GameType   string `json:"game_type" binding:"required"`
		MinPlayers int    `json:"min_players"`
		MaxPlayers int    `json:"max_players"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api.Error(c, apperror.ErrBadRequest.WithData(err.Error()))
		return
	}

	minP := req.MinPlayers
	if minP <= 0 {
		minP = 2
	}
	maxP := req.MaxPlayers
	if maxP <= 0 {
		maxP = 10
	}

	g, err := h.service.CreateGame(req.GameCode, req.GameName, req.GameType, minP, maxP)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Created(c, g)
}

// ListGames handles GET /admin/games
func (h *Handler) ListGames(c *gin.Context) {
	page, pageSize := api.GetPagination(c)
	games, total, err := h.service.ListGames(page, pageSize)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Paginated(c, games, total, page, pageSize)
}

// GetGame handles GET /admin/games/:id
func (h *Handler) GetGame(c *gin.Context) {
	id := parseID(c.Param("id"))
	g, err := h.service.GetGame(id)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Success(c, g)
}

// UpdateGame handles PUT /admin/games/:id
func (h *Handler) UpdateGame(c *gin.Context) {
	id := parseID(c.Param("id"))
	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		api.Error(c, apperror.ErrBadRequest)
		return
	}
	g, err := h.service.UpdateGame(id, updates)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Success(c, g)
}

// DeleteGame handles DELETE /admin/games/:id
func (h *Handler) DeleteGame(c *gin.Context) {
	id := parseID(c.Param("id"))
	if err := h.service.DeleteGame(id); err != nil {
		api.Error(c, err)
		return
	}
	api.SuccessWithMessage(c, "删除成功", nil)
}

// UploadPackage handles POST /admin/games/:id/upload
func (h *Handler) UploadPackage(c *gin.Context) {
	id := parseID(c.Param("id"))
	version := c.PostForm("version")
	if version == "" {
		version = "1.0.0"
	}

	file, _, err := c.Request.FormFile("package")
	if err != nil {
		api.Error(c, apperror.ErrBadRequest.WithMessage("请上传游戏包文件"))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		api.Error(c, apperror.ErrInternalServer)
		return
	}

	gv, err := h.service.UploadPackage(id, version, data)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Created(c, gv)
}

// PublishGame handles POST /admin/games/:id/publish
func (h *Handler) PublishGame(c *gin.Context) {
	id := parseID(c.Param("id"))
	var body struct {
		Version string `json:"version"`
	}
	c.ShouldBindJSON(&body)

	g, err := h.service.PublishGame(id, body.Version)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Success(c, g)
}

// UnpublishGame handles POST /admin/games/:id/unpublish
func (h *Handler) UnpublishGame(c *gin.Context) {
	id := parseID(c.Param("id"))
	if err := h.service.UnpublishGame(id); err != nil {
		api.Error(c, err)
		return
	}
	api.SuccessWithMessage(c, "已下线", nil)
}

func parseID(s string) uint {
	var id uint
	fmt.Sscanf(s, "%d", &id)
	return id
}
