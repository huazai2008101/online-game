package player

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"online-game/pkg/apperror"
)

var (
	ErrPlayerNotFound   = errors.New("player not found")
	ErrPlayerExists     = errors.New("player already exists for this game")
	ErrInvalidLevel     = errors.New("invalid level")
	ErrInvalidExp       = errors.New("invalid experience")
)

// PlayerRepository defines the interface for player data operations
type PlayerRepository interface {
	CreatePlayer(player *Player) error
	GetPlayerByID(id uint) (*Player, error)
	GetPlayerByUserAndGame(userID, gameID uint) (*Player, error)
	ListPlayers(gameID uint, offset, limit int) ([]*Player, int64, error)
	UpdatePlayer(player *Player) error
	DeletePlayer(id uint) error
}

// PlayerStatsRepository defines the interface for player stats operations
type PlayerStatsRepository interface {
	GetOrCreateStats(playerID uint) (*PlayerStats, error)
	UpdateStats(stats *PlayerStats) error
}

// Service provides player business logic
type Service struct {
	playerRepo PlayerRepository
	statsRepo  PlayerStatsRepository
}

// NewService creates a new player service
func NewService(playerRepo PlayerRepository, statsRepo PlayerStatsRepository) *Service {
	return &Service{
		playerRepo: playerRepo,
		statsRepo:  statsRepo,
	}
}

// Player operations

// CreatePlayer creates a new player for a user in a game
func (s *Service) CreatePlayer(ctx context.Context, req *CreatePlayerRequest) (*Player, error) {
	// Validate input
	if req.UserID == 0 {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "user_id", "message": "不能为空"})
	}
	if req.GameID == 0 {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "game_id", "message": "不能为空"})
	}
	if req.Nickname == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "nickname", "message": "不能为空"})
	}

	// Check if player already exists
	existing, _ := s.playerRepo.GetPlayerByUserAndGame(req.UserID, req.GameID)
	if existing != nil {
		return nil, apperror.AlreadyExists("角色")
	}

	player := &Player{
		UserID:   req.UserID,
		GameID:   req.GameID,
		Nickname: req.Nickname,
		Level:    1,
		Exp:      0,
		Score:    0,
		Status:   1,
	}

	if err := s.playerRepo.CreatePlayer(player); err != nil {
		return nil, err
	}

	// Create stats
	_, _ = s.statsRepo.GetOrCreateStats(player.ID)

	return player, nil
}

// GetPlayer retrieves a player by ID
func (s *Service) GetPlayer(ctx context.Context, id uint) (*Player, error) {
	return s.playerRepo.GetPlayerByID(id)
}

// GetPlayerByUserAndGame retrieves a player by user ID and game ID
func (s *Service) GetPlayerByUserAndGame(ctx context.Context, userID, gameID uint) (*Player, error) {
	return s.playerRepo.GetPlayerByUserAndGame(userID, gameID)
}

// ListPlayers lists players with pagination
func (s *Service) ListPlayers(ctx context.Context, gameID uint, page, pageSize int) ([]*Player, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.playerRepo.ListPlayers(gameID, offset, pageSize)
}

// ListPlayersByUserID lists all players for a user
func (s *Service) ListPlayersByUserID(ctx context.Context, userID uint) ([]*Player, error) {
	return s.playerRepo.ListPlayersByUserID(userID)
}

// UpdatePlayer updates a player
func (s *Service) UpdatePlayer(ctx context.Context, id uint, req *UpdatePlayerRequest) error {
	player, err := s.playerRepo.GetPlayerByID(id)
	if err != nil {
		return ErrPlayerNotFound
	}

	if req.Nickname != "" {
		player.Nickname = req.Nickname
	}
	if req.Level != nil {
		if *req.Level < 1 || *req.Level > 1000 {
			return ErrInvalidLevel
		}
		player.Level = *req.Level
	}
	if req.Exp != nil {
		if *req.Exp < 0 {
			return ErrInvalidExp
		}
		player.Exp = *req.Exp
	}
	if req.Score != nil {
		player.Score = *req.Score
	}
	if req.Status != nil {
		player.Status = *req.Status
	}

	return s.playerRepo.UpdatePlayer(player)
}

// DeletePlayer deletes a player
func (s *Service) DeletePlayer(ctx context.Context, id uint) error {
	return s.playerRepo.DeletePlayer(id)
}

// AddExperience adds experience to a player
func (s *Service) AddExperience(ctx context.Context, playerID uint, exp int64) (*Player, error) {
	if exp <= 0 {
		return nil, ErrInvalidExp
	}

	player, err := s.playerRepo.GetPlayerByID(playerID)
	if err != nil {
		return nil, ErrPlayerNotFound
	}

	player.Exp += exp

	// Calculate level up (simple formula: level = sqrt(exp / 100) + 1)
	newLevel := int(player.Exp/1000) + 1
	if newLevel > player.Level {
		player.Level = newLevel
	}

	if err := s.playerRepo.UpdatePlayer(player); err != nil {
		return nil, err
	}

	return player, nil
}

// AddScore adds score to a player
func (s *Service) AddScore(ctx context.Context, playerID uint, score int64) (*Player, error) {
	player, err := s.playerRepo.GetPlayerByID(playerID)
	if err != nil {
		return nil, ErrPlayerNotFound
	}

	player.Score += score

	if err := s.playerRepo.UpdatePlayer(player); err != nil {
		return nil, err
	}

	return player, nil
}

// Stats operations

// GetStats retrieves player statistics
func (s *Service) GetStats(ctx context.Context, playerID uint) (*PlayerStats, error) {
	return s.statsRepo.GetOrCreateStats(playerID)
}

// RecordGameResult records a game result and updates stats
func (s *Service) RecordGameResult(ctx context.Context, playerID uint, won bool, score int64) error {
	stats, err := s.statsRepo.GetOrCreateStats(playerID)
	if err != nil {
		return err
	}

	stats.GamesPlayed++
	if won {
		stats.GamesWon++
	}
	stats.TotalScore += score

	return s.statsRepo.UpdateStats(stats)
}

// Request types

// CreatePlayerRequest represents the request to create a player
type CreatePlayerRequest struct {
	UserID   uint   `json:"user_id" binding:"required"`
	GameID   uint   `json:"game_id" binding:"required"`
	Nickname string `json:"nickname" binding:"required,min=1,max=50"`
}

// UpdatePlayerRequest represents the request to update a player
type UpdatePlayerRequest struct {
	Nickname *string `json:"nickname" binding:"omitempty,min=1,max=50"`
	Level    *int    `json:"level" binding:"omitempty,min=1,max=1000"`
	Exp      *int64  `json:"exp" binding:"omitempty,min=0"`
	Score    *int64  `json:"score" binding:"omitempty"`
	Status   *int    `json:"status" binding:"omitempty,min=0,max=2"`
}

// Repository implementations

// PlayerRepositoryImpl implements PlayerRepository
type PlayerRepositoryImpl struct {
	db *gorm.DB
}

// NewPlayerRepositoryImpl creates a new player repository
func NewPlayerRepositoryImpl(db *gorm.DB) PlayerRepository {
	return &PlayerRepositoryImpl{db: db}
}

func (r *PlayerRepositoryImpl) CreatePlayer(player *Player) error {
	err := r.db.Create(player).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

func (r *PlayerRepositoryImpl) GetPlayerByID(id uint) (*Player, error) {
	var player Player
	err := r.db.First(&player, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrPlayerNotFound
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &player, nil
}

func (r *PlayerRepositoryImpl) GetPlayerByUserAndGame(userID, gameID uint) (*Player, error) {
	var player Player
	err := r.db.Where("user_id = ? AND game_id = ?", userID, gameID).First(&player).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrPlayerNotFound
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &player, nil
}

func (r *PlayerRepositoryImpl) ListPlayers(gameID uint, offset, limit int) ([]*Player, int64, error) {
	var players []*Player
	var total int64

	query := r.db.Model(&Player{})
	if gameID > 0 {
		query = query.Where("game_id = ?", gameID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}

	err := query.Offset(offset).Limit(limit).Order("score DESC, level DESC").Find(&players).Error
	if err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}
	return players, total, nil
}

// ListPlayersByUserID lists all players for a specific user
func (r *PlayerRepositoryImpl) ListPlayersByUserID(userID uint) ([]*Player, error) {
	var players []*Player
	err := r.db.Where("user_id = ?", userID).Find(&players).Error
	if err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return players, nil
}

func (r *PlayerRepositoryImpl) UpdatePlayer(player *Player) error {
	err := r.db.Save(player).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

func (r *PlayerRepositoryImpl) DeletePlayer(id uint) error {
	err := r.db.Delete(&Player{}, id).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

// PlayerStatsRepositoryImpl implements PlayerStatsRepository
type PlayerStatsRepositoryImpl struct {
	db *gorm.DB
}

// NewPlayerStatsRepositoryImpl creates a new player stats repository
func NewPlayerStatsRepositoryImpl(db *gorm.DB) PlayerStatsRepository {
	return &PlayerStatsRepositoryImpl{db: db}
}

func (r *PlayerStatsRepositoryImpl) GetOrCreateStats(playerID uint) (*PlayerStats, error) {
	var stats PlayerStats
	err := r.db.Where("player_id = ?", playerID).First(&stats).Error
	if err == nil {
		return &stats, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	stats = PlayerStats{
		PlayerID:    playerID,
		GamesPlayed: 0,
		GamesWon:    0,
		TotalScore:  0,
	}
	err = r.db.Create(&stats).Error
	return &stats, err
}

func (r *PlayerStatsRepositoryImpl) UpdateStats(stats *PlayerStats) error {
	return r.db.Save(stats).Error
}
