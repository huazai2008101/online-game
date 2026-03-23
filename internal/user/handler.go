package user

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"online-game/pkg/api"
)

// Handler handles HTTP requests for the user service
type Handler struct {
	service *Service
}

// NewHandler creates a new handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all routes for the user service
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	users := r.Group("/users")
	{
		users.POST("/register", h.Register)
		users.POST("/login", h.Login)
		users.POST("/logout", h.Logout)
		users.GET("", h.ListUsers)
		users.GET("/me", h.GetCurrentUser)
		users.GET("/:id", h.GetUser)
		users.PUT("/:id", h.UpdateUser)
		users.DELETE("/:id", h.DeleteUser)
		users.POST("/:id/ban", h.BanUser)
		users.POST("/:id/unban", h.UnbanUser)
		users.POST("/check-username", h.CheckUsername)
		users.POST("/reset-password", h.ResetPassword)
	}

	profiles := r.Group("/profiles")
	{
		profiles.GET("/:id", h.GetProfile)
		profiles.PUT("/:id", h.UpdateProfile)
	}

	password := r.Group("/password")
	{
		password.POST("/change", h.ChangePassword)
	}
}

// RegisterRequest represents the request to register a new user
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=50"`
	Email    string `json:"email" binding:"required,email,max=100"`
	Phone    string `json:"phone" binding:"omitempty,max=20"`
	Nickname string `json:"nickname" binding:"omitempty,max=50"`
}

// LoginRequest represents the request to login
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register registers a new user
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	user, err := h.service.Register(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	// Hide password in response
	user.Password = ""

	api.SuccessWithMessage(c, "注册成功", user)
}

// Login handles user login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	resp, err := h.service.Login(c.Request.Context(), &req)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "登录成功", resp)
}

// Logout handles user logout
func (h *Handler) Logout(c *gin.Context) {
	// TODO: Invalidate JWT token from session
	api.SuccessWithMessage(c, "退出成功", nil)
}

// ListUsers lists users with pagination
func (h *Handler) ListUsers(c *gin.Context) {
	var params api.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		params = api.DefaultPagination()
	}

	users, total, err := h.service.ListUsers(c.Request.Context(), params.Page, params.PerPage)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	// Hide passwords
	for i := range users {
		users[i].Password = ""
	}

	api.Paginated(c, users, params.Page, params.PerPage, total)
}

// GetCurrentUser retrieves the current authenticated user
func (h *Handler) GetCurrentUser(c *gin.Context) {
	// TODO: Get user ID from JWT token
	userID := getUserID(c)

	user, err := h.service.GetProfile(c.Request.Context(), userID)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, user)
}

// GetUser retrieves a user by ID
func (h *Handler) GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	user, err := h.service.GetProfile(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, user)
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Nickname string `json:"nickname" binding:"omitempty,max=50"`
	Avatar   string `json:"avatar" binding:"omitempty,max=255"`
	Phone    string `json:"phone" binding:"omitempty,max=20"`
	Email    string `json:"email" binding:"omitempty,email,max=100"`
}

// UpdateUser updates a user
func (h *Handler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	// Convert to map for UpdateProfile
	updateMap := make(map[string]interface{})
	if req.Nickname != "" {
		updateMap["nickname"] = req.Nickname
	}
	if req.Avatar != "" {
		updateMap["avatar"] = req.Avatar
	}
	if req.Phone != "" {
		updateMap["phone"] = req.Phone
	}
	if req.Email != "" {
		updateMap["email"] = req.Email
	}

	if err := h.service.UpdateProfile(c.Request.Context(), uint(id), updateMap); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "用户更新成功", nil)
}

// DeleteUser deletes a user
func (h *Handler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	if err := h.service.DeleteUser(c.Request.Context(), uint(id)); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "用户删除成功", nil)
}

// BanUser bans a user
func (h *Handler) BanUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	if err := h.service.BanUser(c.Request.Context(), uint(id)); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "用户已封禁", nil)
}

// UnbanUser unbans a user
func (h *Handler) UnbanUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	if err := h.service.UnbanUser(c.Request.Context(), uint(id)); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "用户已解封", nil)
}

// CheckUsernameRequest represents the request to check username availability
type CheckUsernameRequest struct {
	Username string `json:"username" binding:"required"`
}

// CheckUsername checks if a username is available
func (h *Handler) CheckUsername(c *gin.Context) {
	var req CheckUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	available, err := h.service.CheckUsername(c.Request.Context(), req.Username)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, gin.H{
		"username":  req.Username,
		"available": available,
	})
}

// ResetPasswordRequest represents the request to reset password
type ResetPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPassword sends a password reset email
func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	newPassword, err := h.service.ResetPassword(c.Request.Context(), req.Email)
	if err != nil {
		api.HandleError(c, err)
		return
	}

	// In production, send email instead
	api.SuccessWithMessage(c, "密码已重置", gin.H{
		"new_password": newPassword,
		"message":      "生产环境应通过邮件发送",
	})
}

// GetProfile retrieves a user's profile
func (h *Handler) GetProfile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	user, err := h.service.GetProfile(c.Request.Context(), uint(id))
	if err != nil {
		api.HandleError(c, err)
		return
	}

	api.Success(c, user)
}

// UpdateProfileRequest represents the request to update a profile
type UpdateProfileRequest struct {
	Nickname string `json:"nickname" binding:"omitempty,max=50"`
	Avatar   string `json:"avatar" binding:"omitempty,max=255"`
	Phone    string `json:"phone" binding:"omitempty,max=20"`
	Email    string `json:"email" binding:"omitempty,email,max=100"`
}

// UpdateProfile updates a user's profile
func (h *Handler) UpdateProfile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	// Convert to map for UpdateProfile
	updateMap := make(map[string]interface{})
	if req.Nickname != "" {
		updateMap["nickname"] = req.Nickname
	}
	if req.Avatar != "" {
		updateMap["avatar"] = req.Avatar
	}
	if req.Phone != "" {
		updateMap["phone"] = req.Phone
	}
	if req.Email != "" {
		updateMap["email"] = req.Email
	}

	if err := h.service.UpdateProfile(c.Request.Context(), uint(id), updateMap); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "资料更新成功", nil)
}

// ChangePasswordRequest represents the request to change password
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword changes the user's password
func (h *Handler) ChangePassword(c *gin.Context) {
	userID := getUserID(c)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if err := h.service.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		api.HandleError(c, err)
		return
	}

	api.SuccessWithMessage(c, "密码修改成功", nil)
}

// getUserID gets the user ID from context (JWT token)
func getUserID(c *gin.Context) uint {
	// TODO: Get from JWT token
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(uint); ok {
			return id
		}
	}
	return 1 // Default for testing
}
