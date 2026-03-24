# 服务层实现详解

**文档版本:** v1.0
**创建时间:** 2026-03-24
**描述:** 服务层和业务逻辑的完整实现

---

## 目录

1. [用户服务实现](#1-用户服务实现)
2. [游戏服务实现](#2-游戏服务实现)
3. [匹配服务实现](#3-匹配服务实现)
4. [排行榜服务实现](#4-排行榜服务实现)
5. [房间服务实现](#5-房间服务实现)

---

## 1. 用户服务实现

### 1.1 服务接口定义

```go
// internal/user/service/service.go
package service

import (
    "context"
    "time"

    "github.com/your-org/online-game/internal/user/repository"
)

// Service 用户服务接口
type Service interface {
    // Register 用户注册
    Register(ctx context.Context, req *RegisterRequest) (*UserResponse, error)

    // Login 用户登录
    Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)

    // GetProfile 获取用户资料
    GetProfile(ctx context.Context, userID string) (*UserResponse, error)

    // UpdateProfile 更新用户资料
    UpdateProfile(ctx context.Context, userID string, req *UpdateProfileRequest) (*UserResponse, error)

    // ChangePassword 修改密码
    ChangePassword(ctx context.Context, userID string, req *ChangePasswordRequest) error

    // ListUsers 列出用户
    ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error)

    // UpdateStatus 更新用户状态
    UpdateStatus(ctx context.Context, userID string, status int) error

    // DeleteAccount 删除账户
    DeleteAccount(ctx context.Context, userID string) error
}

// RegisterRequest 注册请求
type RegisterRequest struct {
    Username string `json:"username" validate:"required,min=3,max=32,alphanum"`
    Password string `json:"password" validate:"required,min=8"`
    Email    string `json:"email" validate:"required,email"`
    Phone    string `json:"phone" validate:"omitempty,len=11"`
    Nickname string `json:"nickname" validate:"omitempty,max=32"`
}

// LoginRequest 登录请求
type LoginRequest struct {
    Account  string `json:"account" validate:"required"` // 用户名或邮箱
    Password string `json:"password" validate:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
    User      *UserResponse `json:"user"`
    Token     string        `json:"token"`
    RefreshToken string      `json:"refresh_token"`
    ExpiresAt time.Time     `json:"expires_at"`
}

// UserResponse 用户响应
type UserResponse struct {
    ID        string    `json:"id"`
    Username  string    `json:"username"`
    Email     string    `json:"email"`
    Phone     string    `json:"phone,omitempty"`
    Nickname  string    `json:"nickname"`
    Avatar    string    `json:"avatar"`
    Status    int       `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}

// UpdateProfileRequest 更新资料请求
type UpdateProfileRequest struct {
    Nickname string `json:"nickname" validate:"omitempty,max=32"`
    Avatar   string `json:"avatar" validate:"omitempty,url"`
    Phone    string `json:"phone" validate:"omitempty,len=11"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
    OldPassword string `json:"old_password" validate:"required"`
    NewPassword string `json:"new_password" validate:"required,min=8"`
}

// ListUsersRequest 列出用户请求
type ListUsersRequest struct {
    Page     int    `json:"page" validate:"min=1"`
    PageSize int    `json:"page_size" validate:"min=1,max=100"`
    Status   *int   `json:"status,omitempty"`
    Keyword  string `json:"keyword,omitempty"`
}

// ListUsersResponse 列出用户响应
type ListUsersResponse struct {
    Users      []*UserResponse `json:"users"`
    Total      int64           `json:"total"`
    Page       int             `json:"page"`
    PageSize   int             `json:"page_size"`
    TotalPages int             `json:"total_pages"`
}
```

### 1.2 服务实现

```go
// internal/user/service/impl.go
package service

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "fmt"
    "time"

    "github.com/your-org/online-game/internal/user/repository"
    "github.com/your-org/online-game/pkg/apperror"
    "github.com/your-org/online-game/pkg/auth"
    "golang.org/x/crypto/bcrypt"
)

type UserService struct {
    repo       repository.Repository
    jwtManager *auth.JWTManager
}

func NewUserService(repo repository.Repository, jwtManager *auth.JWTManager) Service {
    return &UserService{
        repo:       repo,
        jwtManager: jwtManager,
    }
}

// Register 用户注册
func (s *UserService) Register(ctx context.Context, req *RegisterRequest) (*UserResponse, error) {
    // 验证请求
    if err := s.validateRegister(ctx, req); err != nil {
        return nil, err
    }

    // 检查用户名是否存在
    _, err := s.repo.FindByUsername(ctx, req.Username)
    if err == nil {
        return nil, apperror.ErrUsernameExists.WithMessage("Username already exists")
    }

    // 检查邮箱是否存在
    _, err = s.repo.FindByEmail(ctx, req.Email)
    if err == nil {
        return nil, apperror.ErrEmailExists.WithMessage("Email already exists")
    }

    // 加密密码
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 生成用户ID
    userID := generateUserID()

    // 创建用户
    user := &repository.User{
        ID:       userID,
        Username: req.Username,
        Email:    req.Email,
        Phone:    req.Phone,
        Password: string(hashedPassword),
        Nickname: req.Nickname,
        Status:   1, // 正常状态
    }

    if user.Nickname == "" {
        user.Nickname = user.Username
    }

    if err := s.repo.Create(ctx, user); err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return s.toUserResponse(user), nil
}

// Login 用户登录
func (s *UserService) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
    // 查找用户（支持用户名或邮箱登录）
    var user *repository.User
    var err error

    if isValidEmail(req.Account) {
        user, err = s.repo.FindByEmail(ctx, req.Account)
    } else {
        user, err = s.repo.FindByUsername(ctx, req.Account)
    }

    if err != nil {
        if errors.Is(err, repository.ErrUserNotFound) {
            return nil, apperror.ErrInvalidCredentials
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 验证密码
    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
        return nil, apperror.ErrInvalidCredentials
    }

    // 检查用户状态
    if user.Status != 1 {
        return nil, apperror.ErrAccountDisabled
    }

    // 生成token
    token, err := s.jwtManager.GenerateToken(user.ID, user.Username)
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 生成refresh token
    refreshToken, err := s.generateRefreshToken()
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return &LoginResponse{
        User:         s.toUserResponse(user),
        Token:        token,
        RefreshToken: refreshToken,
        ExpiresAt:    time.Now().Add(24 * time.Hour),
    }, nil
}

// GetProfile 获取用户资料
func (s *UserService) GetProfile(ctx context.Context, userID string) (*UserResponse, error) {
    user, err := s.repo.FindByID(ctx, userID)
    if err != nil {
        if errors.Is(err, repository.ErrUserNotFound) {
            return nil, apperror.ErrUserNotFound
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return s.toUserResponse(user), nil
}

// UpdateProfile 更新用户资料
func (s *UserService) UpdateProfile(ctx context.Context, userID string, req *UpdateProfileRequest) (*UserResponse, error) {
    user, err := s.repo.FindByID(ctx, userID)
    if err != nil {
        if errors.Is(err, repository.ErrUserNotFound) {
            return nil, apperror.ErrUserNotFound
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 更新字段
    if req.Nickname != "" {
        user.Nickname = req.Nickname
    }
    if req.Avatar != "" {
        user.Avatar = req.Avatar
    }
    if req.Phone != "" {
        user.Phone = req.Phone
    }

    if err := s.repo.Update(ctx, user); err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return s.toUserResponse(user), nil
}

// ChangePassword 修改密码
func (s *UserService) ChangePassword(ctx context.Context, userID string, req *ChangePasswordRequest) error {
    user, err := s.repo.FindByID(ctx, userID)
    if err != nil {
        if errors.Is(err, repository.ErrUserNotFound) {
            return apperror.ErrUserNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 验证旧密码
    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
        return apperror.ErrInvalidPassword
    }

    // 加密新密码
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
    if err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    user.Password = string(hashedPassword)

    if err := s.repo.Update(ctx, user); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    return nil
}

// ListUsers 列出用户
func (s *UserService) ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
    // 设置默认值
    if req.Page < 1 {
        req.Page = 1
    }
    if req.PageSize < 1 {
        req.PageSize = 20
    }

    // 构建查询选项
    opts := []repository.ListOption{
        repository.WithLimit(req.PageSize),
        repository.WithOffset((req.Page - 1) * req.PageSize),
    }

    if req.Status != nil {
        opts = append(opts, repository.WithStatus(*req.Status))
    }

    if req.Keyword != "" {
        opts = append(opts, repository.WithKeyword(req.Keyword))
    }

    users, total, err := s.repo.List(ctx, opts...)
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 转换响应
    userResponses := make([]*UserResponse, len(users))
    for i, user := range users {
        userResponses[i] = s.toUserResponse(user)
    }

    totalPages := int(total) / req.PageSize
    if int(total)%req.PageSize > 0 {
        totalPages++
    }

    return &ListUsersResponse{
        Users:      userResponses,
        Total:      total,
        Page:       req.Page,
        PageSize:   req.PageSize,
        TotalPages: totalPages,
    }, nil
}

// UpdateStatus 更新用户状态
func (s *UserService) UpdateStatus(ctx context.Context, userID string, status int) error {
    user, err := s.repo.FindByID(ctx, userID)
    if err != nil {
        if errors.Is(err, repository.ErrUserNotFound) {
            return apperror.ErrUserNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    user.Status = status

    if err := s.repo.Update(ctx, user); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    return nil
}

// DeleteAccount 删除账户
func (s *UserService) DeleteAccount(ctx context.Context, userID string) error {
    err := s.repo.Delete(ctx, userID)
    if err != nil {
        if errors.Is(err, repository.ErrUserNotFound) {
            return apperror.ErrUserNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    return nil
}

// validateRegister 验证注册请求
func (s *UserService) validateRegister(ctx context.Context, req *RegisterRequest) error {
    // 验证用户名格式
    if len(req.Username) < 3 || len(req.Username) > 32 {
        return apperror.ErrInvalidUsername.WithMessage("Username must be 3-32 characters")
    }

    // 验证密码强度
    if len(req.Password) < 8 {
        return apperror.ErrWeakPassword.WithMessage("Password must be at least 8 characters")
    }

    // 验证邮箱格式
    if !isValidEmail(req.Email) {
        return apperror.ErrInvalidEmail
    }

    return nil
}

// toUserResponse 转换为响应
func (s *UserService) toUserResponse(user *repository.User) *UserResponse {
    return &UserResponse{
        ID:        user.ID,
        Username:  user.Username,
        Email:     user.Email,
        Phone:     user.Phone,
        Nickname:  user.Nickname,
        Avatar:    user.Avatar,
        Status:    user.Status,
        CreatedAt: user.CreatedAt,
    }
}

// generateRefreshToken 生成refresh token
func (s *UserService) generateRefreshToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(b), nil
}

// generateUserID 生成用户ID
func generateUserID() string {
    return fmt.Sprintf("U%d", time.Now().UnixNano())
}

// isValidEmail 验证邮箱格式
func isValidEmail(email string) bool {
    // 简单的邮箱验证
    return len(email) > 0 && contains(email, "@")
}

func contains(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}
```

---

## 2. 游戏服务实现

### 2.1 游戏服务接口

```go
// internal/game/service/service.go
package service

import (
    "context"
    "time"
)

// Service 游戏服务接口
type Service interface {
    // CreateGame 创建游戏
    CreateGame(ctx context.Context, req *CreateGameRequest) (*GameResponse, error)

    // GetGame 获取游戏信息
    GetGame(ctx context.Context, gameID string) (*GameResponse, error)

    // ListGames 列出游戏
    ListGames(ctx context.Context, req *ListGamesRequest) (*ListGamesResponse, error)

    // UpdateGame 更新游戏
    UpdateGame(ctx context.Context, gameID string, req *UpdateGameRequest) (*GameResponse, error)

    // DeleteGame 删除游戏
    DeleteGame(ctx context.Context, gameID string) error

    // StartGame 开始游戏
    StartGame(ctx context.Context, gameID string) error

    // StopGame 停止游戏
    StopGame(ctx context.Context, gameID string) error

    // JoinGame 加入游戏
    JoinGame(ctx context.Context, gameID, userID string) (*JoinGameResponse, error)

    // LeaveGame 离开游戏
    LeaveGame(ctx context.Context, gameID, userID string) error

    // SubmitAction 提交游戏动作
    SubmitAction(ctx context.Context, gameID, userID string, action interface{}) error

    // GetGameState 获取游戏状态
    GetGameState(ctx context.Context, gameID string) (interface{}, error)
}

// CreateGameRequest 创建游戏请求
type CreateGameRequest struct {
    Name        string            `json:"name" validate:"required,min=1,max=100"`
    Type        string            `json:"type" validate:"required"`
    Code        string            `json:"code" validate:"required"`
    MaxPlayers  int               `json:"max_players" validate:"min=1,max=100"`
    MinPlayers  int               `json:"min_players" validate:"min=1,max=100"`
    Config      map[string]interface{} `json:"config"`
    Description string            `json:"description" validate:"max=500"`
}

// UpdateGameRequest 更新游戏请求
type UpdateGameRequest struct {
    Name        string            `json:"name" validate:"omitempty,min=1,max=100"`
    Code        string            `json:"code"`
    Config      map[string]interface{} `json:"config"`
    Description string            `json:"description" validate:"omitempty,max=500"`
}

// GameResponse 游戏响应
type GameResponse struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Type        string                 `json:"type"`
    MaxPlayers  int                    `json:"max_players"`
    MinPlayers  int                    `json:"min_players"`
    CurrentPlayers int                 `json:"current_players"`
    Status      string                 `json:"status"`
    Config      map[string]interface{} `json:"config"`
    Description string                 `json:"description"`
    CreatedBy   string                 `json:"created_by"`
    CreatedAt   time.Time              `json:"created_at"`
    StartedAt   *time.Time             `json:"started_at,omitempty"`
    EndedAt     *time.Time             `json:"ended_at,omitempty"`
}

// ListGamesRequest 列出游戏请求
type ListGamesRequest struct {
    Page     int    `json:"page" validate:"min=1"`
    PageSize int    `json:"page_size" validate:"min=1,max=100"`
    Type     string `json:"type,omitempty"`
    Status   string `json:"status,omitempty"`
}

// ListGamesResponse 列出游戏响应
type ListGamesResponse struct {
    Games      []*GameResponse `json:"games"`
    Total      int64           `json:"total"`
    Page       int             `json:"page"`
    PageSize   int             `json:"page_size"`
    TotalPages int             `json:"total_pages"`
}

// JoinGameResponse 加入游戏响应
type JoinGameResponse struct {
    GameID    string      `json:"game_id"`
    PlayerID  string      `json:"player_id"`
    State     interface{} `json:"state"`
}
```

### 2.2 游戏服务实现

```go
// internal/game/service/impl.go
package service

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/your-org/online-game/internal/actor"
    "github.com/your-org/online-game/internal/engine"
    "github.com/your-org/online-game/internal/game/repository"
    "github.com/your-org/online-game/pkg/apperror"
)

type GameService struct {
    repo       repository.Repository
    actorSys   *actor.ActorSystem
    engine     engine.Engine
    hub        *ws.Hub
}

func NewGameService(
    repo repository.Repository,
    actorSys *actor.ActorSystem,
    engine engine.Engine,
    hub *ws.Hub,
) Service {
    return &GameService{
        repo:     repo,
        actorSys: actorSys,
        engine:   engine,
        hub:      hub,
    }
}

// CreateGame 创建游戏
func (s *GameService) CreateGame(ctx context.Context, req *CreateGameRequest) (*GameResponse, error) {
    // 验证请求
    if req.MinPlayers > req.MaxPlayers {
        return nil, apperror.ErrInvalidParameter.WithMessage("min_players cannot be greater than max_players")
    }

    // 加载游戏代码到引擎
    if err := s.engine.LoadGame(ctx, []byte(req.Code)); err != nil {
        return nil, apperror.ErrInvalidGameCode.WithInternal(err)
    }

    // 生成游戏ID
    gameID := generateGameID()

    // 创建游戏记录
    game := &repository.Game{
        ID:          gameID,
        Name:        req.Name,
        Type:        req.Type,
        MaxPlayers:  req.MaxPlayers,
        MinPlayers:  req.MinPlayers,
        Config:      req.Config,
        Description: req.Description,
        Status:      "waiting",
        CreatedBy:   getUserID(ctx),
    }

    if err := s.repo.Create(ctx, game); err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 创建游戏Actor
    _, err := s.actorSys.Spawn(
        fmt.Sprintf("game-%s", gameID),
        "game",
        s.newGameHandler(gameID),
        actor.WithSupervisor(actor.NewFunctionalSupervisor(defaultDecider)),
    )
    if err != nil {
        // 清理已创建的记录
        _ = s.repo.Delete(ctx, gameID)
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return s.toGameResponse(game), nil
}

// GetGame 获取游戏信息
func (s *GameService) GetGame(ctx context.Context, gameID string) (*GameResponse, error) {
    game, err := s.repo.FindByID(ctx, gameID)
    if err != nil {
        if errors.Is(err, repository.ErrGameNotFound) {
            return nil, apperror.ErrGameNotFound
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return s.toGameResponse(game), nil
}

// ListGames 列出游戏
func (s *GameService) ListGames(ctx context.Context, req *ListGamesRequest) (*ListGamesResponse, error) {
    if req.Page < 1 {
        req.Page = 1
    }
    if req.PageSize < 1 {
        req.PageSize = 20
    }

    games, total, err := s.repo.List(ctx, repository.ListOptions{
        Limit:    req.PageSize,
        Offset:   (req.Page - 1) * req.PageSize,
        Type:     req.Type,
        Status:   req.Status,
    })
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    gameResponses := make([]*GameResponse, len(games))
    for i, game := range games {
        gameResponses[i] = s.toGameResponse(game)
    }

    totalPages := int(total) / req.PageSize
    if int(total)%req.PageSize > 0 {
        totalPages++
    }

    return &ListGamesResponse{
        Games:      gameResponses,
        Total:      total,
        Page:       req.Page,
        PageSize:   req.PageSize,
        TotalPages: totalPages,
    }, nil
}

// UpdateGame 更新游戏
func (s *GameService) UpdateGame(ctx context.Context, gameID string, req *UpdateGameRequest) (*GameResponse, error) {
    game, err := s.repo.FindByID(ctx, gameID)
    if err != nil {
        if errors.Is(err, repository.ErrGameNotFound) {
            return nil, apperror.ErrGameNotFound
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 只有waiting状态的游戏可以更新
    if game.Status != "waiting" {
        return nil, apperror.ErrGameNotEditable
    }

    // 更新字段
    if req.Name != "" {
        game.Name = req.Name
    }
    if req.Code != "" {
        game.Code = req.Code
        // 重新加载代码到引擎
        if err := s.engine.LoadGame(ctx, []byte(req.Code)); err != nil {
            return nil, apperror.ErrInvalidGameCode.WithInternal(err)
        }
    }
    if req.Config != nil {
        game.Config = req.Config
    }
    if req.Description != "" {
        game.Description = req.Description
    }

    if err := s.repo.Update(ctx, game); err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return s.toGameResponse(game), nil
}

// DeleteGame 删除游戏
func (s *GameService) DeleteGame(ctx context.Context, gameID string) error {
    game, err := s.repo.FindByID(ctx, gameID)
    if err != nil {
        if errors.Is(err, repository.ErrGameNotFound) {
            return apperror.ErrGameNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 只有waiting或ended状态的游戏可以删除
    if game.Status != "waiting" && game.Status != "ended" {
        return apperror.ErrGameNotDeletable
    }

    // 停止游戏Actor
    actorID := fmt.Sprintf("game-%s", gameID)
    _ = s.actorSys.StopActor(actorID)

    // 删除游戏记录
    if err := s.repo.Delete(ctx, gameID); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    return nil
}

// StartGame 开始游戏
func (s *GameService) StartGame(ctx context.Context, gameID string) error {
    game, err := s.repo.FindByID(ctx, gameID)
    if err != nil {
        if errors.Is(err, repository.ErrGameNotFound) {
            return apperror.ErrGameNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 检查游戏状态
    if game.Status != "waiting" {
        return apperror.ErrGameNotStartable
    }

    // 检查玩家数量
    if game.CurrentPlayers < game.MinPlayers {
        return apperror.ErrNotEnoughPlayers
    }

    // 更新游戏状态
    now := time.Now()
    game.Status = "playing"
    game.StartedAt = &now

    if err := s.repo.Update(ctx, game); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 通知游戏Actor开始
    actorID := fmt.Sprintf("game-%s", gameID)
    s.actorSys.Tell(actorID, &actor.StringMessage{
        Type: "start",
        Data: gameID,
    })

    // 启动引擎
    if err := s.engine.Start(ctx); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 通知所有玩家
    s.hub.BroadcastToRoom(gameID, map[string]interface{}{
        "type":    "game_started",
        "game_id": gameID,
    })

    return nil
}

// StopGame 停止游戏
func (s *GameService) StopGame(ctx context.Context, gameID string) error {
    game, err := s.repo.FindByID(ctx, gameID)
    if err != nil {
        if errors.Is(err, repository.ErrGameNotFound) {
            return apperror.ErrGameNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 只有playing状态的游戏可以停止
    if game.Status != "playing" {
        return apperror.ErrGameNotStoppable
    }

    // 更新游戏状态
    now := time.Now()
    game.Status = "ended"
    game.EndedAt = &now

    if err := s.repo.Update(ctx, game); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 停止引擎
    if err := s.engine.Stop(ctx); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 通知所有玩家
    s.hub.BroadcastToRoom(gameID, map[string]interface{}{
        "type":    "game_ended",
        "game_id": gameID,
    })

    return nil
}

// JoinGame 加入游戏
func (s *GameService) JoinGame(ctx context.Context, gameID, userID string) (*JoinGameResponse, error) {
    game, err := s.repo.FindByID(ctx, gameID)
    if err != nil {
        if errors.Is(err, repository.ErrGameNotFound) {
            return nil, apperror.ErrGameNotFound
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 检查游戏状态
    if game.Status != "waiting" {
        return nil, apperror.ErrGameNotJoinable
    }

    // 检查玩家数量
    if game.CurrentPlayers >= game.MaxPlayers {
        return nil, apperror.ErrGameFull
    }

    // 检查玩家是否已加入
    if s.repo.IsPlayerInGame(ctx, gameID, userID) {
        return nil, apperror.ErrAlreadyInGame
    }

    // 加入游戏
    player := &repository.Player{
        ID:        generatePlayerID(),
        GameID:    gameID,
        UserID:    userID,
        Status:    "active",
        JoinedAt:  time.Now(),
    }

    if err := s.repo.AddPlayer(ctx, player); err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 更新游戏玩家数
    game.CurrentPlayers++
    if err := s.repo.Update(ctx, game); err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 获取游戏状态
    state, _ := s.engine.GetState(ctx)

    // 通知游戏Actor
    actorID := fmt.Sprintf("game-%s", gameID)
    s.actorSys.Tell(actorID, &actor.StringMessage{
        Type: "player_joined",
        Data: map[string]interface{}{
            "player_id": userID,
        },
    })

    return &JoinGameResponse{
        GameID:   gameID,
        PlayerID: player.ID,
        State:    state,
    }, nil
}

// LeaveGame 离开游戏
func (s *GameService) LeaveGame(ctx context.Context, gameID, userID string) error {
    game, err := s.repo.FindByID(ctx, gameID)
    if err != nil {
        if errors.Is(err, repository.ErrGameNotFound) {
            return apperror.ErrGameNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 检查玩家是否在游戏中
    if !s.repo.IsPlayerInGame(ctx, gameID, userID) {
        return apperror.ErrNotInGame
    }

    // 移除玩家
    if err := s.repo.RemovePlayer(ctx, gameID, userID); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 更新游戏玩家数
    game.CurrentPlayers--
    if err := s.repo.Update(ctx, game); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 通知游戏Actor
    actorID := fmt.Sprintf("game-%s", gameID)
    s.actorSys.Tell(actorID, &actor.StringMessage{
        Type: "player_left",
        Data: map[string]interface{}{
            "player_id": userID,
        },
    })

    return nil
}

// SubmitAction 提交游戏动作
func (s *GameService) SubmitAction(ctx context.Context, gameID, userID string, action interface{}) error {
    game, err := s.repo.FindByID(ctx, gameID)
    if err != nil {
        if errors.Is(err, repository.ErrGameNotFound) {
            return apperror.ErrGameNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 检查游戏状态
    if game.Status != "playing" {
        return apperror.ErrGameNotPlaying
    }

    // 检查玩家是否在游戏中
    if !s.repo.IsPlayerInGame(ctx, gameID, userID) {
        return apperror.ErrNotInGame
    }

    // 调用引擎处理动作
    _, err = s.engine.Call(ctx, "handleAction", userID, action)
    if err != nil {
        return apperror.ErrInvalidAction.WithInternal(err)
    }

    return nil
}

// GetGameState 获取游戏状态
func (s *GameService) GetGameState(ctx context.Context, gameID string) (interface{}, error) {
    return s.engine.GetState(ctx)
}

// newGameHandler 创建游戏Actor处理器
func (s *GameService) newGameHandler(gameID string) actor.Handler {
    return func(ctx context.Context, msg actor.Message) error {
        switch msg.Type() {
        case "start":
            return s.handleGameStart(ctx, gameID)
        case "player_joined":
            return s.handlePlayerJoined(ctx, gameID, msg)
        case "player_left":
            return s.handlePlayerLeft(ctx, gameID, msg)
        default:
            return fmt.Errorf("unknown message type: %s", msg.Type())
        }
    }
}

func (s *GameService) handleGameStart(ctx context.Context, gameID string) error {
    // 调用游戏引擎的onStart
    _, err := s.engine.Call(ctx, "onStart")
    return err
}

func (s *GameService) handlePlayerJoined(ctx context.Context, gameID string, msg actor.Message) error {
    // 调用游戏引擎的onPlayerJoined
    data := msg.(*actor.StringMessage).Data.(map[string]interface{})
    _, err := s.engine.Call(ctx, "onPlayerJoined", data["player_id"])
    return err
}

func (s *GameService) handlePlayerLeft(ctx context.Context, gameID string, msg actor.Message) error {
    // 调用游戏引擎的onPlayerLeft
    data := msg.(*actor.StringMessage).Data.(map[string]interface{})
    _, err := s.engine.Call(ctx, "onPlayerLeft", data["player_id"])
    return err
}

func (s *GameService) toGameResponse(game *repository.Game) *GameResponse {
    return &GameResponse{
        ID:             game.ID,
        Name:           game.Name,
        Type:           game.Type,
        MaxPlayers:     game.MaxPlayers,
        MinPlayers:     game.MinPlayers,
        CurrentPlayers: game.CurrentPlayers,
        Status:         game.Status,
        Config:         game.Config,
        Description:    game.Description,
        CreatedBy:      game.CreatedBy,
        CreatedAt:      game.CreatedAt,
        StartedAt:      game.StartedAt,
        EndedAt:        game.EndedAt,
    }
}

func generateGameID() string {
    return fmt.Sprintf("G%d", time.Now().UnixNano())
}

func generatePlayerID() string {
    return fmt.Sprintf("P%d", time.Now().UnixNano())
}

func getUserID(ctx context.Context) string {
    if userID, ok := ctx.Value("user_id").(string); ok {
        return userID
    }
    return ""
}

var defaultDecider = func(err error) actor.Directive {
    return actor.Restart
}
```

---

## 3. 匹配服务实现

### 3.1 匹配服务接口

```go
// internal/match/service/service.go
package service

import (
    "context"
)

// Service 匹配服务接口
type Service interface {
    // JoinQueue 加入匹配队列
    JoinQueue(ctx context.Context, req *JoinQueueRequest) (*JoinQueueResponse, error)

    // LeaveQueue 离开匹配队列
    LeaveQueue(ctx context.Context, queueID, userID string) error

    // GetQueueStatus 获取队列状态
    GetQueueStatus(ctx context.Context, queueID string) (*QueueStatusResponse, error)

    // CreateMatch 创建匹配
    CreateMatch(ctx context.Context, req *CreateMatchRequest) (*MatchResponse, error)

    // CancelMatch 取消匹配
    CancelMatch(ctx context.Context, matchID string) error
}

// JoinQueueRequest 加入队列请求
type JoinQueueRequest struct {
    QueueID  string                 `json:"queue_id" validate:"required"`
    UserID   string                 `json:"user_id" validate:"required"`
    GameType string                 `json:"game_type" validate:"required"`
    Rank     int                    `json:"rank"`
    Metadata map[string]interface{} `json:"metadata"`
}

// JoinQueueResponse 加入队列响应
type JoinQueueResponse struct {
    TicketID   string `json:"ticket_id"`
    QueueID    string `json:"queue_id"`
    Position   int    `json:"position"`
    EstimatedWait int `json:"estimated_wait"`
}

// QueueStatusResponse 队列状态响应
type QueueStatusResponse struct {
    QueueID    string `json:"queue_id"`
    PlayerCount int    `json:"player_count"`
    AvgWaitTime int    `json:"avg_wait_time"`
}

// CreateMatchRequest 创建匹配请求
type CreateMatchRequest struct {
    Name       string   `json:"name" validate:"required"`
    GameType   string   `json:"game_type" validate:"required"`
    MaxPlayers int      `json:"max_players" validate:"min=2,max=100"`
    MinPlayers int      `json:"min_players" validate:"min=2,max=100"`
    RankRange  RankRange `json:"rank_range"`
}

// RankRange 排位范围
type RankRange struct {
    Min int `json:"min"`
    Max int `json:"max"`
}

// MatchResponse 匹配响应
type MatchResponse struct {
    MatchID  string   `json:"match_id"`
    Name     string   `json:"name"`
    GameType string   `json:"game_type"`
    Players  []string `json:"players"`
    Status   string   `json:"status"`
}
```

### 3.2 匹配服务实现

```go
// internal/match/service/impl.go
package service

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/your-org/online-game/internal/match/repository"
    "github.com/your-org/online-game/pkg/apperror"
)

type MatchService struct {
    repo       repository.Repository
    queues     map[string]*MatchQueue
    queueMutex sync.RWMutex
    matcher    Matcher
}

func NewMatchService(repo repository.Repository, matcher Matcher) Service {
    return &MatchService{
        repo:    repo,
        queues:  make(map[string]*MatchQueue),
        matcher: matcher,
    }
}

// JoinQueue 加入匹配队列
func (s *MatchService) JoinQueue(ctx context.Context, req *JoinQueueRequest) (*JoinQueueResponse, error) {
    s.queueMutex.Lock()
    defer s.queueMutex.Unlock()

    // 获取或创建队列
    queue, ok := s.queues[req.QueueID]
    if !ok {
        // 从数据库加载队列配置
        queueConfig, err := s.repo.GetQueue(ctx, req.QueueID)
        if err != nil {
            return nil, apperror.ErrQueueNotFound
        }

        queue = NewMatchQueue(queueConfig)
        s.queues[req.QueueID] = queue
    }

    // 创建票据
    ticket := &MatchTicket{
        ID:        generateTicketID(),
        UserID:    req.UserID,
        GameType:  req.GameType,
        Rank:      req.Rank,
        Metadata:  req.Metadata,
        JoinedAt:  time.Now(),
    }

    // 加入队列
    position, estimatedWait := queue.Join(ticket)

    return &JoinQueueResponse{
        TicketID:      ticket.ID,
        QueueID:       req.QueueID,
        Position:      position,
        EstimatedWait: estimatedWait,
    }, nil
}

// LeaveQueue 离开匹配队列
func (s *MatchService) LeaveQueue(ctx context.Context, queueID, userID string) error {
    s.queueMutex.Lock()
    defer s.queueMutex.Unlock()

    queue, ok := s.queues[queueID]
    if !ok {
        return apperror.ErrQueueNotFound
    }

    queue.RemoveByUserID(userID)
    return nil
}

// GetQueueStatus 获取队列状态
func (s *MatchService) GetQueueStatus(ctx context.Context, queueID string) (*QueueStatusResponse, error) {
    s.queueMutex.RLock()
    defer s.queueMutex.RUnlock()

    queue, ok := s.queues[queueID]
    if !ok {
        return nil, apperror.ErrQueueNotFound
    }

    stats := queue.Stats()
    return &QueueStatusResponse{
        QueueID:     queueID,
        PlayerCount: stats.PlayerCount,
        AvgWaitTime: stats.AvgWaitTime,
    }, nil
}

// CreateMatch 创建匹配
func (s *MatchService) CreateMatch(ctx context.Context, req *CreateMatchRequest) (*MatchResponse, error) {
    match := &repository.Match{
        ID:         generateMatchID(),
        Name:       req.Name,
        GameType:   req.GameType,
        MaxPlayers: req.MaxPlayers,
        MinPlayers: req.MinPlayers,
        RankMin:    req.RankRange.Min,
        RankMax:    req.RankRange.Max,
        Status:     "waiting",
        CreatedAt:  time.Now(),
    }

    if err := s.repo.CreateMatch(ctx, match); err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return &MatchResponse{
        MatchID:  match.ID,
        Name:     match.Name,
        GameType: match.GameType,
        Status:   match.Status,
    }, nil
}

// CancelMatch 取消匹配
func (s *MatchService) CancelMatch(ctx context.Context, matchID string) error {
    match, err := s.repo.GetMatch(ctx, matchID)
    if err != nil {
        if errors.Is(err, repository.ErrMatchNotFound) {
            return apperror.ErrMatchNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    if match.Status != "waiting" {
        return apperror.ErrMatchNotCancellable
    }

    match.Status = "cancelled"
    match.CancelledAt = time.Now()

    if err := s.repo.UpdateMatch(ctx, match); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    return nil
}

// MatchQueue 匹配队列
type MatchQueue struct {
    config      *repository.QueueConfig
    tickets     map[string]*MatchTicket
    ticketMutex sync.RWMutex
    waitTimes   []time.Duration
}

// NewMatchQueue 创建匹配队列
func NewMatchQueue(config *repository.QueueConfig) *MatchQueue {
    return &MatchQueue{
        config:  config,
        tickets: make(map[string]*MatchTicket),
    }
}

// Join 加入队列
func (q *MatchQueue) Join(ticket *MatchTicket) (position int, estimatedWait int) {
    q.ticketMutex.Lock()
    defer q.ticketMutex.Unlock()

    q.tickets[ticket.ID] = ticket
    position = len(q.tickets)

    // 计算预计等待时间
    estimatedWait = q.calculateEstimatedWait()

    // 启动匹配
    go q.tryMatch()

    return position, estimatedWait
}

// RemoveByUserID 根据用户ID移除
func (q *MatchQueue) RemoveByUserID(userID string) {
    q.ticketMutex.Lock()
    defer q.ticketMutex.Unlock()

    for id, ticket := range q.tickets {
        if ticket.UserID == userID {
            delete(q.tickets, id)
            return
        }
    }
}

// Stats 返回统计信息
func (q *MatchQueue) Stats() *QueueStats {
    q.ticketMutex.RLock()
    defer q.ticketMutex.RUnlock()

    avgWait := 0
    if len(q.waitTimes) > 0 {
        total := 0
        for _, wt := range q.waitTimes {
            total += int(wt.Seconds())
        }
        avgWait = total / len(q.waitTimes)
    }

    return &QueueStats{
        PlayerCount: len(q.tickets),
        AvgWaitTime: avgWait,
    }
}

// tryMatch 尝试匹配
func (q *MatchQueue) tryMatch() {
    q.ticketMutex.Lock()
    tickets := make([]*MatchTicket, 0, len(q.tickets))
    for _, ticket := range q.tickets {
        tickets = append(tickets, ticket)
    }
    q.ticketMutex.Unlock()

    // 使用匹配器进行匹配
    matches := q.matcher.Match(tickets, q.config)

    // 处理匹配结果
    for _, match := range matches {
        // 移除已匹配的票据
        q.ticketMutex.Lock()
        for _, ticket := range match.Tickets {
            delete(q.tickets, ticket.ID)
            // 记录等待时间
            q.waitTimes = append(q.waitTimes, time.Since(ticket.JoinedAt))
        }
        q.ticketMutex.Unlock()

        // 通知匹配成功
        q.notifyMatch(match)
    }
}

// calculateEstimatedWait 计算预计等待时间
func (q *MatchQueue) calculateEstimatedWait() int {
    if len(q.waitTimes) == 0 {
        return 30 // 默认30秒
    }

    total := 0
    for _, wt := range q.waitTimes {
        total += int(wt.Seconds())
    }
    return total / len(q.waitTimes)
}

// notifyMatch 通知匹配结果
func (q *MatchQueue) notifyMatch(match *Match) {
    // TODO: 通过WebSocket通知玩家
}

// QueueStats 队列统计
type QueueStats struct {
    PlayerCount int
    AvgWaitTime int
}

// MatchTicket 匹配票据
type MatchTicket struct {
    ID       string
    UserID   string
    GameType string
    Rank     int
    Metadata map[string]interface{}
    JoinedAt time.Time
}

// Match 匹配结果
type Match struct {
    ID      string
    Tickets []*MatchTicket
}

// Matcher 匹配器接口
type Matcher interface {
    Match(tickets []*MatchTicket, config *repository.QueueConfig) []*Match
}

// RankMatcher 排位匹配器
type RankMatcher struct{}

func NewRankMatcher() Matcher {
    return &RankMatcher{}
}

func (m *RankMatcher) Match(tickets []*MatchTicket, config *repository.QueueConfig) []*Match {
    // 按排位分组
    rankGroups := make(map[int][]*MatchTicket)
    for _, ticket := range tickets {
        rank := ticket.Rank / 100 * 100 // 每100分一组
        rankGroups[rank] = append(rankGroups[rank], ticket)
    }

    var matches []*Match
    matchID := 0

    // 检查每组是否有足够的玩家
    for _, group := range rankGroups {
        if len(group) >= config.MinPlayers {
            // 创建匹配
            matchSize := min(len(group), config.MaxPlayers)
            matchTickets := group[:matchSize]

            matches = append(matches, &Match{
                ID:      fmt.Sprintf("M%d", matchID),
                Tickets: matchTickets,
            })

            matchID++
        }
    }

    return matches
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func generateTicketID() string {
    return fmt.Sprintf("T%d", time.Now().UnixNano())
}

func generateMatchID() string {
    return fmt.Sprintf("M%d", time.Now().UnixNano())
}
```

---

## 4. 排行榜服务实现

### 4.1 排行榜服务接口

```go
// internal/leaderboard/service/service.go
package service

import (
    "context"
)

// Service 排行榜服务接口
type Service interface {
    // GetLeaderboard 获取排行榜
    GetLeaderboard(ctx context.Context, req *GetLeaderboardRequest) (*LeaderboardResponse, error)

    // UpdateScore 更新分数
    UpdateScore(ctx context.Context, req *UpdateScoreRequest) error

    // GetRank 获取排名
    GetRank(ctx context.Context, req *GetRankRequest) (*RankResponse, error)

    // CreateLeaderboard 创建排行榜
    CreateLeaderboard(ctx context.Context, req *CreateLeaderboardRequest) (*LeaderboardInfo, error)

    // DeleteLeaderboard 删除排行榜
    DeleteLeaderboard(ctx context.Context, leaderboardID string) error
}

// GetLeaderboardRequest 获取排行榜请求
type GetLeaderboardRequest struct {
    LeaderboardID string `json:"leaderboard_id" validate:"required"`
    Page          int    `json:"page" validate:"min=1"`
    PageSize      int    `json:"page_size" validate:"min=1,max=100"`
}

// LeaderboardResponse 排行榜响应
type LeaderboardResponse struct {
    LeaderboardID string           `json:"leaderboard_id"`
    Name          string           `json:"name"`
    Entries       []*LeaderboardEntry `json:"entries"`
    Total         int64            `json:"total"`
    Page          int              `json:"page"`
    PageSize      int              `json:"page_size"`
    UpdatedAt     time.Time        `json:"updated_at"`
}

// LeaderboardEntry 排行榜条目
type LeaderboardEntry struct {
    Rank        int       `json:"rank"`
    UserID      string    `json:"user_id"`
    Username    string    `json:"username"`
    Score       int64     `json:"score"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// UpdateScoreRequest 更新分数请求
type UpdateScoreRequest struct {
    LeaderboardID string                 `json:"leaderboard_id" validate:"required"`
    UserID        string                 `json:"user_id" validate:"required"`
    Score         int64                  `json:"score" validate:"min=0"`
    Metadata      map[string]interface{} `json:"metadata"`
}

// GetRankRequest 获取排名请求
type GetRankRequest struct {
    LeaderboardID string `json:"leaderboard_id" validate:"required"`
    UserID        string `json:"user_id" validate:"required"`
}

// RankResponse 排名响应
type RankResponse struct {
    Rank   int   `json:"rank"`
    Score  int64 `json:"score"`
    Count  int64 `json:"count"`
}

// CreateLeaderboardRequest 创建排行榜请求
type CreateLeaderboardRequest struct {
    Name        string `json:"name" validate:"required,min=1,max=100"`
    Type        string `json:"type" validate:"required,oneof=daily weekly monthly alltime"`
    GameType    string `json:"game_type" validate:"required"`
    Description string `json:"description" validate:"max=500"`
}

// LeaderboardInfo 排行榜信息
type LeaderboardInfo struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Type        string    `json:"type"`
    GameType    string    `json:"game_type"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
}
```

### 4.2 排行榜服务实现（使用Redis Sorted Set）

```go
// internal/leaderboard/service/impl.go
package service

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/your-org/online-game/internal/leaderboard/repository"
    "github.com/your-org/online-game/pkg/apperror"
)

type LeaderboardService struct {
    repo   repository.Repository
    redis  *redis.Client
}

func NewLeaderboardService(repo repository.Repository, redis *redis.Client) Service {
    return &LeaderboardService{
        repo:  repo,
        redis: redis,
    }
}

// GetLeaderboard 获取排行榜
func (s *LeaderboardService) GetLeaderboard(ctx context.Context, req *GetLeaderboardRequest) (*LeaderboardResponse, error) {
    if req.Page < 1 {
        req.Page = 1
    }
    if req.PageSize < 1 {
        req.PageSize = 20
    }

    // 获取排行榜信息
    leaderboard, err := s.repo.GetByID(ctx, req.LeaderboardID)
    if err != nil {
        if errors.Is(err, repository.ErrLeaderboardNotFound) {
            return nil, apperror.ErrLeaderboardNotFound
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // Redis key
    key := s.getLeaderboardKey(req.LeaderboardID)

    // 获取总数
    total, err := s.redis.ZCard(ctx, key).Result()
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 获取排名范围（倒序，从高到低）
    start := int64((req.Page - 1) * req.PageSize)
    stop := start + int64(req.PageSize) - 1

    results, err := s.redis.ZRevRangeWithScores(ctx, key, start, stop).Result()
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 获取用户信息
    userIDs := make([]string, len(results))
    for i, result := range results {
        userIDs[i] = result.Member.(string)
    }

    users, err := s.repo.GetUsersByIDs(ctx, userIDs)
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    userMap := make(map[string]*repository.User)
    for _, user := range users {
        userMap[user.ID] = user
    }

    // 构建响应
    entries := make([]*LeaderboardEntry, len(results))
    for i, result := range results {
        userID := result.Member.(string)
        user := userMap[userID]

        entries[i] = &LeaderboardEntry{
            Rank:      int(start) + i + 1,
            UserID:    userID,
            Username:  user.Username,
            Score:     int64(result.Score),
            UpdatedAt: time.Now(),
        }
    }

    return &LeaderboardResponse{
        LeaderboardID: req.LeaderboardID,
        Name:          leaderboard.Name,
        Entries:       entries,
        Total:         total,
        Page:          req.Page,
        PageSize:      req.PageSize,
        UpdatedAt:     leaderboard.UpdatedAt,
    }, nil
}

// UpdateScore 更新分数
func (s *LeaderboardService) UpdateScore(ctx context.Context, req *UpdateScoreRequest) error {
    // 验证排行榜存在
    _, err := s.repo.GetByID(ctx, req.LeaderboardID)
    if err != nil {
        if errors.Is(err, repository.ErrLeaderboardNotFound) {
            return apperror.ErrLeaderboardNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // Redis key
    key := s.getLeaderboardKey(req.LeaderboardID)

    // 更新分数
    if err := s.redis.ZAdd(ctx, key, redis.Z{
        Score:  float64(req.Score),
        Member: req.UserID,
    }).Err(); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 设置过期时间（根据类型）
    ttl := s.getTTLByType(leaderboard.Type)
    if ttl > 0 {
        s.redis.Expire(ctx, key, ttl)
    }

    // 更新排行榜更新时间
    leaderboard.UpdatedAt = time.Now()
    if err := s.repo.Update(ctx, leaderboard); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    return nil
}

// GetRank 获取排名
func (s *LeaderboardService) GetRank(ctx context.Context, req *GetRankRequest) (*RankResponse, error) {
    // 验证排行榜存在
    leaderboard, err := s.repo.GetByID(ctx, req.LeaderboardID)
    if err != nil {
        if errors.Is(err, repository.ErrLeaderboardNotFound) {
            return nil, apperror.ErrLeaderboardNotFound
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    key := s.getLeaderboardKey(req.LeaderboardID)

    // 获取排名（倒序）
    rank, err := s.redis.ZRevRank(ctx, key, req.UserID).Result()
    if err != nil {
        if err == redis.Nil {
            return &RankResponse{
                Rank:  0,
                Score: 0,
                Count: 0,
            }, nil
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 获取分数
    score, err := s.redis.ZScore(ctx, key, req.UserID).Result()
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 获取总数
    count, err := s.redis.ZCard(ctx, key).Result()
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return &RankResponse{
        Rank:  int(rank) + 1, // Redis rank从0开始
        Score: int64(score),
        Count: count,
    }, nil
}

// CreateLeaderboard 创建排行榜
func (s *LeaderboardService) CreateLeaderboard(ctx context.Context, req *CreateLeaderboardRequest) (*LeaderboardInfo, error) {
    leaderboard := &repository.Leaderboard{
        ID:          generateLeaderboardID(),
        Name:        req.Name,
        Type:        req.Type,
        GameType:    req.GameType,
        Description: req.Description,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }

    if err := s.repo.Create(ctx, leaderboard); err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return &LeaderboardInfo{
        ID:          leaderboard.ID,
        Name:        leaderboard.Name,
        Type:        leaderboard.Type,
        GameType:    leaderboard.GameType,
        Description: leaderboard.Description,
        CreatedAt:   leaderboard.CreatedAt,
    }, nil
}

// DeleteLeaderboard 删除排行榜
func (s *LeaderboardService) DeleteLeaderboard(ctx context.Context, leaderboardID string) error {
    // 删除数据库记录
    if err := s.repo.Delete(ctx, leaderboardID); err != nil {
        if errors.Is(err, repository.ErrLeaderboardNotFound) {
            return apperror.ErrLeaderboardNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 删除Redis数据
    key := s.getLeaderboardKey(leaderboardID)
    s.redis.Del(ctx, key)

    return nil
}

// getLeaderboardKey 获取Redis key
func (s *LeaderboardService) getLeaderboardKey(leaderboardID string) string {
    return fmt.Sprintf("leaderboard:%s", leaderboardID)
}

// getTTLByType 根据类型获取TTL
func (s *LeaderboardService) getTTLByType(leaderboardType string) time.Duration {
    switch leaderboardType {
    case "daily":
        return 24 * time.Hour
    case "weekly":
        return 7 * 24 * time.Hour
    case "monthly":
        return 30 * 24 * time.Hour
    case "alltime":
        return 0 // 永久
    default:
        return 24 * time.Hour
    }
}

func generateLeaderboardID() string {
    return fmt.Sprintf("L%d", time.Now().UnixNano())
}
```

---

## 5. 房间服务实现

### 5.1 房间服务接口

```go
// internal/room/service/service.go
package service

import (
    "context"
)

// Service 房间服务接口
type Service interface {
    // CreateRoom 创建房间
    CreateRoom(ctx context.Context, req *CreateRoomRequest) (*RoomResponse, error)

    // GetRoom 获取房间信息
    GetRoom(ctx context.Context, roomID string) (*RoomResponse, error)

    // ListRooms 列出房间
    ListRooms(ctx context.Context, req *ListRoomsRequest) (*ListRoomsResponse, error)

    // JoinRoom 加入房间
    JoinRoom(ctx context.Context, roomID, userID string, password string) error

    // LeaveRoom 离开房间
    LeaveRoom(ctx context.Context, roomID, userID string) error

    // UpdateRoom 更新房间
    UpdateRoom(ctx context.Context, roomID string, req *UpdateRoomRequest) (*RoomResponse, error)

    // DeleteRoom 删除房间
    DeleteRoom(ctx context.Context, roomID string) error

    // SendMessage 发送房间消息
    SendMessage(ctx context.Context, roomID, userID string, content string) error

    // KickMember 踢出成员
    KickMember(ctx context.Context, roomID, ownerID, memberID string) error
}

// CreateRoomRequest 创建房间请求
type CreateRoomRequest struct {
    Name        string                 `json:"name" validate:"required,min=1,max=50"`
    Type        string                 `json:"type" validate:"required"`
    MaxPlayers  int                    `json:"max_players" validate:"min=2,max=100"`
    Password    string                 `json:"password" validate:"omitempty,max=20"`
    IsPrivate   bool                   `json:"is_private"`
    Config      map[string]interface{} `json:"config"`
}

// UpdateRoomRequest 更新房间请求
type UpdateRoomRequest struct {
    Name       string                 `json:"name" validate:"omitempty,min=1,max=50"`
    Password   string                 `json:"password" validate:"omitempty,max=20"`
    IsPrivate  *bool                  `json:"is_private"`
    Config     map[string]interface{} `json:"config"`
}

// RoomResponse 房间响应
type RoomResponse struct {
    ID           string                 `json:"id"`
    Name         string                 `json:"name"`
    Type         string                 `json:"type"`
    MaxPlayers   int                    `json:"max_players"`
    CurrentCount int                    `json:"current_count"`
    IsPrivate    bool                   `json:"is_private"`
    HasPassword  bool                   `json:"has_password"`
    Config       map[string]interface{} `json:"config"`
    OwnerID      string                 `json:"owner_id"`
    Members      []*RoomMember          `json:"members"`
    CreatedAt    time.Time              `json:"created_at"`
}

// RoomMember 房间成员
type RoomMember struct {
    UserID    string    `json:"user_id"`
    Username  string    `json:"username"`
    Avatar    string    `json:"avatar"`
    Role      string    `json:"role"` // owner, admin, member
    JoinedAt  time.Time `json:"joined_at"`
    IsReady   bool      `json:"is_ready"`
}

// ListRoomsRequest 列出房间请求
type ListRoomsRequest struct {
    Page     int    `json:"page" validate:"min=1"`
    PageSize int    `json:"page_size" validate:"min=1,max=100"`
    Type     string `json:"type,omitempty"`
    Status   string `json:"status,omitempty"` // waiting, playing, full
}
```

### 5.2 房间服务实现

```go
// internal/room/service/impl.go
package service

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/your-org/online-game/internal/room/repository"
    "github.com/your-org/online-game/internal/ws"
    "github.com/your-org/online-game/pkg/apperror"
    "golang.org/x/crypto/bcrypt"
)

type RoomService struct {
    repo  repository.Repository
    hub   *ws.Hub
}

func NewRoomService(repo repository.Repository, hub *ws.Hub) Service {
    return &RoomService{
        repo: repo,
        hub:  hub,
    }
}

// CreateRoom 创建房间
func (s *RoomService) CreateRoom(ctx context.Context, req *CreateRoomRequest) (*RoomResponse, error) {
    userID := getUserID(ctx)

    // 验证请求
    if req.MaxPlayers < 2 {
        return nil, apperror.ErrInvalidParameter.WithMessage("max_players must be at least 2")
    }

    // 加密密码（如果有）
    var hashedPassword string
    if req.Password != "" {
        hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
        if err != nil {
            return nil, apperror.ErrInternal.WithInternal(err)
        }
        hashedPassword = string(hash)
    }

    // 创建房间
    room := &repository.Room{
        ID:          generateRoomID(),
        Name:        req.Name,
        Type:        req.Type,
        MaxPlayers:  req.MaxPlayers,
        Password:    hashedPassword,
        IsPrivate:   req.IsPrivate,
        Config:      req.Config,
        OwnerID:     userID,
        Status:      "waiting",
        CreatedAt:   time.Now(),
    }

    if err := s.repo.Create(ctx, room); err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 房主自动加入
    member := &repository.RoomMember{
        RoomID:   room.ID,
        UserID:   userID,
        Role:     "owner",
        JoinedAt: time.Now(),
    }

    if err := s.repo.AddMember(ctx, member); err != nil {
        // 回滚房间创建
        _ = s.repo.Delete(ctx, room.ID)
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 创建WebSocket房间
    wsRoom := s.hub.GetOrCreateRoom(room.ID)

    // 获取房间信息返回
    return s.GetRoom(ctx, room.ID)
}

// GetRoom 获取房间信息
func (s *RoomService) GetRoom(ctx context.Context, roomID string) (*RoomResponse, error) {
    room, err := s.repo.FindByID(ctx, roomID)
    if err != nil {
        if errors.Is(err, repository.ErrRoomNotFound) {
            return nil, apperror.ErrRoomNotFound
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 获取成员
    members, err := s.repo.GetMembers(ctx, roomID)
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 转换成员响应
    memberResponses := make([]*RoomMember, len(members))
    for i, m := range members {
        memberResponses[i] = &RoomMember{
            UserID:   m.UserID,
            Username: m.Username,
            Avatar:   m.Avatar,
            Role:     m.Role,
            JoinedAt: m.JoinedAt,
            IsReady:  m.IsReady,
        }
    }

    return &RoomResponse{
        ID:           room.ID,
        Name:         room.Name,
        Type:         room.Type,
        MaxPlayers:   room.MaxPlayers,
        CurrentCount: len(members),
        IsPrivate:    room.IsPrivate,
        HasPassword:  room.Password != "",
        Config:       room.Config,
        OwnerID:      room.OwnerID,
        Members:      memberResponses,
        CreatedAt:    room.CreatedAt,
    }, nil
}

// ListRooms 列出房间
func (s *RoomService) ListRooms(ctx context.Context, req *ListRoomsRequest) (*ListRoomsResponse, error) {
    if req.Page < 1 {
        req.Page = 1
    }
    if req.PageSize < 1 {
        req.PageSize = 20
    }

    rooms, total, err := s.repo.List(ctx, repository.ListOptions{
        Limit:  req.PageSize,
        Offset: (req.Page - 1) * req.PageSize,
        Type:   req.Type,
        Status: req.Status,
    })
    if err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    roomResponses := make([]*RoomResponse, len(rooms))
    for i, room := range rooms {
        // 获取成员数
        members, _ := s.repo.GetMembers(ctx, room.ID)

        roomResponses[i] = &RoomResponse{
            ID:           room.ID,
            Name:         room.Name,
            Type:         room.Type,
            MaxPlayers:   room.MaxPlayers,
            CurrentCount: len(members),
            IsPrivate:    room.IsPrivate,
            HasPassword:  room.Password != "",
            Config:       room.Config,
            OwnerID:      room.OwnerID,
            Members:      []*RoomMember{}, // 列表时不返回详细成员
            CreatedAt:    room.CreatedAt,
        }
    }

    totalPages := int(total) / req.PageSize
    if int(total)%req.PageSize > 0 {
        totalPages++
    }

    return &ListRoomsResponse{
        Rooms:      roomResponses,
        Total:      total,
        Page:       req.Page,
        PageSize:   req.PageSize,
        TotalPages: totalPages,
    }, nil
}

// JoinRoom 加入房间
func (s *RoomService) JoinRoom(ctx context.Context, roomID, userID, password string) error {
    room, err := s.repo.FindByID(ctx, roomID)
    if err != nil {
        if errors.Is(err, repository.ErrRoomNotFound) {
            return apperror.ErrRoomNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 检查房间状态
    if room.Status != "waiting" {
        return apperror.ErrRoomNotJoinable
    }

    // 检查密码
    if room.Password != "" {
        if password == "" {
            return apperror.ErrRoomPasswordRequired
        }
        if err := bcrypt.CompareHashAndPassword([]byte(room.Password), []byte(password)); err != nil {
            return apperror.ErrInvalidRoomPassword
        }
    }

    // 检查是否已加入
    if s.repo.IsMember(ctx, roomID, userID) {
        return apperror.ErrAlreadyInRoom
    }

    // 检查人数
    members, _ := s.repo.GetMembers(ctx, roomID)
    if len(members) >= room.MaxPlayers {
        return apperror.ErrRoomFull
    }

    // 加入房间
    member := &repository.RoomMember{
        RoomID:   roomID,
        UserID:   userID,
        Role:     "member",
        JoinedAt: time.Now(),
    }

    if err := s.repo.AddMember(ctx, member); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 通知房间成员
    s.hub.BroadcastToRoom(roomID, map[string]interface{}{
        "type":    "member_joined",
        "room_id": roomID,
        "user_id": userID,
    })

    return nil
}

// LeaveRoom 离开房间
func (s *RoomService) LeaveRoom(ctx context.Context, roomID, userID string) error {
    room, err := s.repo.FindByID(ctx, roomID)
    if err != nil {
        if errors.Is(err, repository.ErrRoomNotFound) {
            return apperror.ErrRoomNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 检查是否是房主
    if room.OwnerID == userID {
        // 房主离开，转让或关闭房间
        members, _ := s.repo.GetMembers(ctx, roomID)
        if len(members) > 1 {
            // 转让给下一个加入的成员
            newOwnerID := ""
            for _, m := range members {
                if m.UserID != userID {
                    newOwnerID = m.UserID
                    break
                }
            }
            if newOwnerID != "" {
                room.OwnerID = newOwnerID
                _ = s.repo.Update(ctx, room)
            }
        } else {
            // 关闭房间
            _ = s.repo.Delete(ctx, roomID)
        }
    }

    // 移除成员
    if err := s.repo.RemoveMember(ctx, roomID, userID); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 通知房间成员
    s.hub.BroadcastToRoom(roomID, map[string]interface{}{
        "type":    "member_left",
        "room_id": roomID,
        "user_id": userID,
    })

    return nil
}

// UpdateRoom 更新房间
func (s *RoomService) UpdateRoom(ctx context.Context, roomID string, req *UpdateRoomRequest) (*RoomResponse, error) {
    userID := getUserID(ctx)

    room, err := s.repo.FindByID(ctx, roomID)
    if err != nil {
        if errors.Is(err, repository.ErrRoomNotFound) {
            return nil, apperror.ErrRoomNotFound
        }
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    // 只有房主可以更新
    if room.OwnerID != userID {
        return nil, apperror.ErrNotRoomOwner
    }

    // 更新字段
    if req.Name != "" {
        room.Name = req.Name
    }
    if req.Password != "" {
        hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
        if err != nil {
            return nil, apperror.ErrInternal.WithInternal(err)
        }
        room.Password = string(hash)
    }
    if req.IsPrivate != nil {
        room.IsPrivate = *req.IsPrivate
    }
    if req.Config != nil {
        room.Config = req.Config
    }

    if err := s.repo.Update(ctx, room); err != nil {
        return nil, apperror.ErrInternal.WithInternal(err)
    }

    return s.GetRoom(ctx, roomID)
}

// DeleteRoom 删除房间
func (s *RoomService) DeleteRoom(ctx context.Context, roomID string) error {
    userID := getUserID(ctx)

    room, err := s.repo.FindByID(ctx, roomID)
    if err != nil {
        if errors.Is(err, repository.ErrRoomNotFound) {
            return apperror.ErrRoomNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 只有房主可以删除
    if room.OwnerID != userID {
        return apperror.ErrNotRoomOwner
    }

    // 删除房间
    if err := s.repo.Delete(ctx, roomID); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 通知房间成员
    s.hub.BroadcastToRoom(roomID, map[string]interface{}{
        "type":    "room_closed",
        "room_id": roomID,
    })

    return nil
}

// SendMessage 发送房间消息
func (s *RoomService) SendMessage(ctx context.Context, roomID, userID, content string) error {
    // 检查是否是成员
    if !s.repo.IsMember(ctx, roomID, userID) {
        return apperror.ErrNotInRoom
    }

    // 广播消息
    s.hub.BroadcastToRoom(roomID, map[string]interface{}{
        "type":      "chat",
        "room_id":   roomID,
        "user_id":   userID,
        "content":   content,
        "timestamp": time.Now().Unix(),
    })

    return nil
}

// KickMember 踢出成员
func (s *RoomService) KickMember(ctx context.Context, roomID, ownerID, memberID string) error {
    room, err := s.repo.FindByID(ctx, roomID)
    if err != nil {
        if errors.Is(err, repository.ErrRoomNotFound) {
            return apperror.ErrRoomNotFound
        }
        return apperror.ErrInternal.WithInternal(err)
    }

    // 验证是房主
    if room.OwnerID != ownerID {
        return apperror.ErrNotRoomOwner
    }

    // 不能踢出房主
    if memberID == room.OwnerID {
        return apperror.ErrCannotKickOwner
    }

    // 移除成员
    if err := s.repo.RemoveMember(ctx, roomID, memberID); err != nil {
        return apperror.ErrInternal.WithInternal(err)
    }

    // 通知被踢出的用户
    s.hub.SendToUser(memberID, []byte(fmt.Sprintf(`{"type":"kicked","room_id":"%s"}`, roomID)))

    // 通知房间成员
    s.hub.BroadcastToRoom(roomID, map[string]interface{}{
        "type":      "member_kicked",
        "room_id":   roomID,
        "user_id":   memberID,
    })

    return nil
}

// ListRoomsResponse 列出房间响应
type ListRoomsResponse struct {
    Rooms      []*RoomResponse `json:"rooms"`
    Total      int64           `json:"total"`
    Page       int             `json:"page"`
    PageSize   int             `json:"page_size"`
    TotalPages int             `json:"total_pages"`
}

func getUserID(ctx context.Context) string {
    if userID, ok := ctx.Value("user_id").(string); ok {
        return userID
    }
    return ""
}

func generateRoomID() string {
    return fmt.Sprintf("R%d", time.Now().UnixNano())
}
```
