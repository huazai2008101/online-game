package user

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"online-game/pkg/api"
	"online-game/pkg/apperror"
	"online-game/pkg/auth"
)

// Handler handles HTTP requests for user service.
type Handler struct {
	service *Service
}

// NewHandler creates a new user handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers user routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	auth := rg.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
	}

	users := rg.Group("/users")
	users.Use(authMiddleware(h.service))
	{
		users.GET("/me", h.GetCurrentUser)
		users.GET("/:id", h.GetUser)
	}
}

// Register handles POST /auth/register
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.Error(c, apperror.ErrBadRequest.WithData(err.Error()))
		return
	}

	user, err := h.service.Register(&req)
	if err != nil {
		api.Error(c, err)
		return
	}

	api.Created(c, user)
}

// Login handles POST /auth/login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.Error(c, apperror.ErrBadRequest.WithData(err.Error()))
		return
	}

	resp, err := h.service.Login(&req)
	if err != nil {
		api.Error(c, err)
		return
	}

	api.Success(c, resp)
}

// GetCurrentUser handles GET /users/me
func (h *Handler) GetCurrentUser(c *gin.Context) {
	userID := getUserID(c)
	if userID == 0 {
		api.Error(c, apperror.ErrUnauthorized)
		return
	}

	user, err := h.service.GetUser(userID)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Success(c, user)
}

// GetUser handles GET /users/:id
func (h *Handler) GetUser(c *gin.Context) {
	var uri struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		api.Error(c, apperror.ErrBadRequest.WithData(err.Error()))
		return
	}

	info, err := h.service.GetUserInfo(uri.ID)
	if err != nil {
		api.Error(c, err)
		return
	}
	api.Success(c, info)
}

// authMiddleware validates JWT tokens.
func authMiddleware(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, api.Response{
				Code:    40100,
				Message: "缺少认证Token",
			})
			return
		}

		claims, err := svc.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, api.Response{
				Code:    40100,
				Message: "Token无效或已过期",
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

// extractToken extracts JWT from Authorization header.
func extractToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}

// getUserID extracts user ID from gin context.
func getUserID(c *gin.Context) uint {
	if id, exists := c.Get("user_id"); exists {
		if uid, ok := id.(uint); ok {
			return uid
		}
	}
	return 0
}

// GetClaimsFromContext extracts JWT claims from gin context.
func GetClaimsFromContext(c *gin.Context) *auth.Claims {
	if claims, exists := c.Get("claims"); exists {
		if c, ok := claims.(*auth.Claims); ok {
			return c
		}
	}
	return nil
}
