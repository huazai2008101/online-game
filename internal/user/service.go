package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"online-game/pkg/apperror"
	"online-game/pkg/auth"
)

// Service handles user business logic.
type Service struct {
	db         *gorm.DB
	jwtManager *auth.JWTManager
	cache      *goredis.Client // optional: nil = no caching
}

// NewService creates a new user service.
func NewService(db *gorm.DB, jwtManager *auth.JWTManager) *Service {
	return &Service{db: db, jwtManager: jwtManager}
}

// SetCache sets the Redis cache client.
func (s *Service) SetCache(cache *goredis.Client) {
	s.cache = cache
}

// Register creates a new user account.
func (s *Service) Register(req *RegisterRequest) (*User, error) {
	// Check if username exists
	var count int64
	s.db.Model(&User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		return nil, apperror.ErrUserExists.WithMessage("用户名已存在")
	}

	// Check if email exists
	s.db.Model(&User{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		return nil, apperror.ErrUserExists.WithMessage("邮箱已被注册")
	}

	// Hash password
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	user := &User{
		Username: req.Username,
		Password: string(hashedPwd),
		Nickname: nickname,
		Email:    req.Email,
		Status:   1,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, apperror.ErrDatabaseError.WithData(err.Error())
	}
	return user, nil
}

// Login authenticates a user and returns a JWT token.
func (s *Service) Login(req *LoginRequest) (*LoginResponse, error) {
	var user User
	if err := s.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrUserNotFound
		}
		return nil, apperror.ErrDatabaseError.WithData(err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, apperror.ErrInvalidPassword
	}

	if user.Status == 0 {
		return nil, apperror.ErrForbidden.WithMessage("账号已被封禁")
	}

	role := "player"
	if user.Status == 2 {
		role = "admin"
	}

	token, expiresAt, err := s.jwtManager.GenerateToken(user.ID, user.Username, role)
	if err != nil {
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}

	// Save session
	session := &UserSession{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: expiresAt,
	}
	s.db.Create(session)

	return &LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      &user,
	}, nil
}

// GetUser retrieves a user by ID.
func (s *Service) GetUser(id uint) (*User, error) {
	// Try cache
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:%d", id)
	if s.cache != nil {
		cached, err := s.cache.Get(ctx, cacheKey).Result()
		if err == nil {
			var user User
			if json.Unmarshal([]byte(cached), &user) == nil {
				return &user, nil
			}
		}
	}

	var user User
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrUserNotFound
		}
		return nil, apperror.ErrDatabaseError.WithData(err.Error())
	}

	// Cache for 30 minutes
	if s.cache != nil {
		if data, err := json.Marshal(&user); err == nil {
			s.cache.Set(ctx, cacheKey, data, 30*time.Minute)
		}
	}
	return &user, nil
}

// ValidateToken validates a JWT token and returns claims.
func (s *Service) ValidateToken(tokenStr string) (*auth.Claims, error) {
	claims, err := s.jwtManager.ValidateToken(tokenStr)
	if err != nil {
		return nil, apperror.ErrInvalidToken
	}
	return claims, nil
}

// ListUsers returns a paginated user list.
func (s *Service) ListUsers(page, pageSize int) ([]User, int64, error) {
	var users []User
	var total int64

	s.db.Model(&User{}).Count(&total)
	offset := (page - 1) * pageSize
	if err := s.db.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, apperror.ErrDatabaseError.WithData(err.Error())
	}
	return users, total, nil
}

// Migrate runs auto migration for user tables.
func (s *Service) Migrate() error {
	return s.db.AutoMigrate(&User{}, &UserSession{})
}

// GetUserInfo returns public user info.
func (s *Service) GetUserInfo(id uint) (*UserInfo, error) {
	user, err := s.GetUser(id)
	if err != nil {
		return nil, err
	}
	return &UserInfo{
		ID:       user.ID,
		Username: user.Username,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Status:   user.Status,
	}, nil
}

// Ensure Service implements ValidateToken for gRPC.
var _ = fmt.Sprintf // avoid unused import
