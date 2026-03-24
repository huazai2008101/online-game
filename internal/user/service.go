package user

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"online-game/pkg/apperror"
	"online-game/pkg/auth"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrUserExists          = errors.New("user already exists")
	ErrInvalidCredentials  = errors.New("invalid username or password")
	ErrInvalidToken        = errors.New("invalid token")
)

// UserRepository defines user data operations
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id uint) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByPhone(ctx context.Context, phone string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, page, pageSize int) ([]User, int64, error)
}

// userRepository implements UserRepository
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *User) error {
	err := r.db.WithContext(ctx).Create(user).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrUserNotFound
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &user, nil
}

func (r *userRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrUserNotFound
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrUserNotFound
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &user, nil
}

func (r *userRepository) FindByPhone(ctx context.Context, phone string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("phone = ?", phone).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrUserNotFound
		}
		return nil, apperror.ErrInternalServer.WithData(err.Error())
	}
	return &user, nil
}

func (r *userRepository) Update(ctx context.Context, user *User) error {
	err := r.db.WithContext(ctx).Save(user).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id uint) error {
	err := r.db.WithContext(ctx).Delete(&User{}, id).Error
	if err != nil {
		return apperror.ErrInternalServer.WithData(err.Error())
	}
	return nil
}

func (r *userRepository) List(ctx context.Context, page, pageSize int) ([]User, int64, error) {
	var users []User
	var total int64

	err := r.db.WithContext(ctx).Model(&User{}).Count(&total).Error
	if err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}

	offset := (page - 1) * pageSize
	err = r.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&users).Error
	if err != nil {
		return nil, 0, apperror.ErrInternalServer.WithData(err.Error())
	}
	return users, total, nil
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret     string
	ExpireTime time.Duration
	Issuer     string
}

// Service provides user business logic
type Service struct {
	repo      UserRepository
	jwtConfig *JWTConfig
}

// NewService creates a new user service
func NewService(repo UserRepository) *Service {
	return &Service{
		repo: repo,
		jwtConfig: &JWTConfig{
			Secret:     getEnv("JWT_SECRET", "online-game-secret-key-change-in-production"),
			ExpireTime: 24 * time.Hour,
			Issuer:     "online-game",
		},
	}
}

// NewServiceWithJWT creates a new user service with custom JWT config
func NewServiceWithJWT(repo UserRepository, jwtConfig *JWTConfig) *Service {
	return &Service{
		repo:      repo,
		jwtConfig: jwtConfig,
	}
}

// LoginResponse represents login response
type LoginResponse struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Token    string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// Register registers a new user
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*User, error) {
	// Validate input
	if req.Username == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "username", "message": "不能为空"})
	}
	if len(req.Password) < 6 {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "password", "message": "密码长度至少6位"})
	}
	if req.Email == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "email", "message": "不能为空"})
	}

	// Check if username exists
	existing, err := s.repo.FindByUsername(ctx, req.Username)
	if err == nil && existing != nil {
		return nil, apperror.ErrUserAlreadyExists
	}

	// Check if email exists
	if req.Email != "" {
		existing, err = s.repo.FindByEmail(ctx, req.Email)
		if err == nil && existing != nil {
			return nil, apperror.ErrUserAlreadyExists
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperror.ErrInternalServer.WithMessage("密码加密失败")
	}

	// Generate user ID
	userID := uint(time.Now().Unix())

	// Create user
	user := &User{
		ID:       userID,
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Phone:    req.Phone,
		Nickname: req.Nickname,
		Status:   1,
	}

	if user.Nickname == "" {
		user.Nickname = user.Username
	}

	// Create user profile
	profile := &UserProfile{
		UserID: user.ID,
	}

	// Save to database
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	_ = profile
	return user, nil
}

// Login authenticates a user
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// Validate input
	if req.Username == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "username", "message": "不能为空"})
	}
	if req.Password == "" {
		return nil, apperror.ErrBadRequest.WithData(map[string]string{"field": "password", "message": "不能为空"})
	}

	user, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, apperror.ErrInvalidPassword
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, apperror.ErrInvalidPassword
	}

	// Check account status
	if user.Status != 1 {
		return nil, apperror.ErrForbidden.WithMessage("账号已被禁用")
	}

	// Generate JWT token
	token, expiresAt, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	user.UpdatedAt = time.Now()

	return &LoginResponse{
		UserID:   user.ID,
		Username: user.Username,
		Nickname: user.Nickname,
		Token:    token,
		ExpiresAt: expiresAt,
	}, nil
}

// GetProfile retrieves user profile
func (s *Service) GetProfile(ctx context.Context, userID uint) (*User, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.Password = ""
	return user, nil
}

// UpdateProfile updates user profile
func (s *Service) UpdateProfile(ctx context.Context, userID uint, req map[string]interface{}) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	// Update allowed fields
	if nickname, ok := req["nickname"]; ok {
		if s, ok := nickname.(string); ok {
			if len(s) > 50 {
				return apperror.ErrBadRequest.WithData(map[string]string{"field": "nickname", "message": "长度不能超过50"})
			}
			user.Nickname = s
		}
	}

	if avatar, ok := req["avatar"]; ok {
		if s, ok := avatar.(string); ok {
			user.Avatar = s
		}
	}

	if phone, ok := req["phone"]; ok {
		if s, ok := phone.(string); ok {
			user.Phone = s
		}
	}

	if email, ok := req["email"]; ok {
		if s, ok := email.(string); ok {
			user.Email = s
		}
	}

	return s.repo.Update(ctx, user)
}

// ChangePassword changes user password
func (s *Service) ChangePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	if oldPassword == "" {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "old_password", "message": "不能为空"})
	}
	if len(newPassword) < 6 {
		return apperror.ErrBadRequest.WithData(map[string]string{"field": "new_password", "message": "密码长度至少6位"})
	}

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return apperror.ErrInvalidPassword
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return apperror.ErrInternalServer.WithMessage("密码加密失败")
	}

	user.Password = string(hashedPassword)
	return s.repo.Update(ctx, user)
}

// generateToken generates a JWT token for a user
func (s *Service) generateToken(user *User) (string, int64, error) {
	now := time.Now()
	exp := now.Add(s.jwtConfig.ExpireTime)

	claims := &auth.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     s.getRole(user),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.jwtConfig.Issuer,
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtConfig.Secret))
	if err != nil {
		return "", 0, apperror.ErrInternalServer.WithMessage("生成令牌失败")
	}

	return tokenString, exp.Unix(), nil
}

// validateToken validates a JWT token
func (s *Service) validateToken(tokenString string) (*auth.Claims, error) {
	config := &auth.JWTConfig{
		Secret:     s.jwtConfig.Secret,
		ExpireTime: s.jwtConfig.ExpireTime,
		Issuer:     s.jwtConfig.Issuer,
	}

	claims, err := auth.ParseToken(config, tokenString)
	if err != nil {
		return nil, apperror.ErrUnauthorized.WithMessage(err.Error())
	}

	return claims, nil
}

// getRole determines the user's role
func (s *Service) getRole(user *User) string {
	if user.Status == 2 {
		return "admin"
	}
	return "user"
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// ListUsers lists users with pagination
func (s *Service) ListUsers(ctx context.Context, page, pageSize int) ([]User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.List(ctx, page, pageSize)
}

// DeleteUser deletes a user
func (s *Service) DeleteUser(ctx context.Context, userID uint) error {
	return s.repo.Delete(ctx, userID)
}

// BanUser bans a user
func (s *Service) BanUser(ctx context.Context, userID uint) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	user.Status = 0
	return s.repo.Update(ctx, user)
}

// UnbanUser unbans a user
func (s *Service) UnbanUser(ctx context.Context, userID uint) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	user.Status = 1
	return s.repo.Update(ctx, user)
}

// CheckUsername checks if username is available
func (s *Service) CheckUsername(ctx context.Context, username string) (bool, error) {
	if username == "" {
		return false, apperror.ErrBadRequest.WithData(map[string]string{"field": "username", "message": "不能为空"})
	}

	_, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return true, nil
		}
		return false, err
	}
	return false, apperror.ErrUserAlreadyExists
}

// ResetPassword resets user password
func (s *Service) ResetPassword(ctx context.Context, email string) (string, error) {
	if email == "" {
		return "", apperror.ErrBadRequest.WithData(map[string]string{"field": "email", "message": "不能为空"})
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return "", err
	}

	// Generate random password
	newPassword := generateRandomPassword(12)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", apperror.ErrInternalServer.WithMessage("密码加密失败")
	}

	user.Password = string(hashedPassword)
	if err := s.repo.Update(ctx, user); err != nil {
		return "", err
	}

	return newPassword, nil
}

// generateRandomPassword generates a random password
func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	max := big.NewInt(int64(len(charset)))
	for i := range b {
		n, _ := cryptorand.Int(cryptorand.Reader, max)
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
