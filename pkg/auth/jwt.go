// Package auth provides JWT authentication middleware
package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenExpired     = errors.New("token expired")
	ErrTokenInvalid     = errors.New("token invalid")
	ErrTokenMissing     = errors.New("token missing")
	ErrUnauthorized     = errors.New("unauthorized")
)

// Claims represents JWT claims
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret     string
	ExpireTime time.Duration
	Issuer     string
}

// DefaultJWTConfig returns default JWT configuration
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		Secret:     "online-game-secret-key-change-in-production",
		ExpireTime: 24 * time.Hour,
		Issuer:     "online-game",
	}
}

// NewJWTConfig creates a new JWT configuration
func NewJWTConfig(secret string, expireTime time.Duration) *JWTConfig {
	return &JWTConfig{
		Secret:     secret,
		ExpireTime: expireTime,
		Issuer:     "online-game",
	}
}

// GenerateToken generates a JWT token
func GenerateToken(config *JWTConfig, userID uint, username, role string) (string, int64, error) {
	now := time.Now()
	exp := now.Add(config.ExpireTime)
	expireTime := exp.Unix()

	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.Secret))
	if err != nil {
		return "", 0, err
	}

	return tokenString, expireTime, nil
}

// ParseToken parses a JWT token
func ParseToken(config *JWTConfig, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Secret), nil
	})

	if err != nil {
		return nil, ErrTokenInvalid
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrTokenInvalid
}

// RefreshToken refreshes a JWT token
func RefreshToken(config *JWTConfig, tokenString string) (string, int64, error) {
	claims, err := ParseToken(config, tokenString)
	if err != nil {
		return "", 0, err
	}

	return GenerateToken(config, claims.UserID, claims.Username, claims.Role)
}

// Middleware returns a JWT authentication middleware
func Middleware(config *JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "missing authorization header",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid authorization format",
			})
			return
		}

		claims, err := ParseToken(config, parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid or expired token",
			})
			return
		}

		// Set claims in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

// OptionalMiddleware returns an optional JWT authentication middleware
// It allows requests to proceed without authentication, but will set user info if token is provided
func OptionalMiddleware(config *JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := ParseToken(config, parts[1])
		if err != nil {
			c.Next()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

// RequireRole returns a middleware that requires specific roles
func RequireRole(config *JWTConfig, allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "missing authorization header",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid authorization format",
			})
			return
		}

		claims, err := ParseToken(config, parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid or expired token",
			})
			return
		}

		// Check role
		allowed := false
		for _, role := range allowedRoles {
			if claims.Role == role {
				allowed = true
				break
			}
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "insufficient permissions",
			})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

// GetUserID returns the user ID from context
func GetUserID(c *gin.Context) uint {
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(uint); ok {
			return id
		}
	}
	return 0
}

// GetUsername returns the username from context
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		if name, ok := username.(string); ok {
			return name
		}
	}
	return ""
}

// GetRole returns the role from context
func GetRole(c *gin.Context) string {
	if role, exists := c.Get("role"); exists {
		if r, ok := role.(string); ok {
			return r
		}
	}
	return ""
}

// GetClaims returns the full claims from context
func GetClaims(c *gin.Context) *Claims {
	if claims, exists := c.Get("claims"); exists {
		if c, ok := claims.(*Claims); ok {
			return c
		}
	}
	return nil
}
