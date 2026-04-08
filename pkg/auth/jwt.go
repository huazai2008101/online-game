package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	goredis "github.com/redis/go-redis/v9"
)

// Claims extends JWT standard claims with user info.
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT creation and validation.
type JWTManager struct {
	secret     []byte
	expiration time.Duration
	redis      *goredis.Client // optional: nil = no blacklist check
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(secret string, expiration time.Duration) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

// SetRedisClient sets the Redis client for token blacklist support.
func (m *JWTManager) SetRedisClient(client *goredis.Client) {
	m.redis = client
}

// GenerateToken creates a new JWT token for a user.
func (m *JWTManager) GenerateToken(userID uint, username, role string) (string, time.Time, error) {
	expiresAt := time.Now().Add(m.expiration)
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "online-game",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return tokenStr, expiresAt, nil
}

// ValidateToken parses and validates a JWT token.
// Also checks the Redis blacklist if available.
func (m *JWTManager) ValidateToken(tokenStr string) (*Claims, error) {
	// Check blacklist first
	if m.redis != nil {
		blacklisted, err := m.IsBlacklisted(tokenStr)
		if err == nil && blacklisted {
			return nil, fmt.Errorf("token has been revoked")
		}
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// BlacklistToken adds a token to the blacklist.
// TTL is set to the remaining token lifetime.
func (m *JWTManager) BlacklistToken(tokenStr string) error {
	if m.redis == nil {
		return nil // no Redis, skip blacklist
	}

	// Parse token to get expiry
	claims, err := m.ValidateToken(tokenStr)
	if err != nil {
		return err // token already invalid, nothing to blacklist
	}

	key := tokenBlacklistKey(tokenStr)
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil // already expired
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return m.redis.Set(ctx, key, "1", ttl).Err()
}

// IsBlacklisted checks if a token is in the blacklist.
func (m *JWTManager) IsBlacklisted(tokenStr string) (bool, error) {
	if m.redis == nil {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	n, err := m.redis.Exists(ctx, tokenBlacklistKey(tokenStr)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// tokenBlacklistKey returns the Redis key for a blacklisted token.
// Uses SHA256 hash to avoid storing the raw token.
func tokenBlacklistKey(tokenStr string) string {
	hash := sha256.Sum256([]byte(tokenStr))
	return "jwt_blacklist:" + hex.EncodeToString(hash[:])
}

// DefaultSecret returns the JWT secret from env or a default (dev only).
func DefaultSecret() string {
	return "online-game-jwt-secret-change-in-prod"
}
