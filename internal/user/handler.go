package user

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"online-game/pkg/api"
)

// Handler handles HTTP requests for the user service
type Handler struct {
	repo *Repository
}

// NewHandler creates a new handler
func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
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
	}

	profiles := r.Group("/profiles")
	{
		profiles.GET("/:id", h.GetProfile)
		profiles.PUT("/:id", h.UpdateProfile)
	}

	friends := r.Group("/friends")
	{
		friends.POST("", h.AddFriend)
		friends.GET("", h.GetFriends)
		friends.PUT("/:friend_id", h.UpdateFriend)
		friends.DELETE("/:friend_id", h.DeleteFriend)
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

	// Check if username exists
	if _, err := h.repo.GetUserByUsername(req.Username); err == nil {
		api.BadRequest(c, "用户名已存在")
		return
	}

	// Check if email exists
	if _, err := h.repo.GetUserByEmail(req.Email); err == nil {
		api.BadRequest(c, "邮箱已被注册")
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		api.InternalError(c, "密码加密失败")
		return
	}

	user := &User{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Phone:    req.Phone,
		Nickname: req.Nickname,
		Status:   1,
	}

	if err := h.repo.CreateUser(user); err != nil {
		api.InternalError(c, "创建用户失败")
		return
	}

	// Create profile
	profile := &UserProfile{UserID: user.ID}
	_ = h.repo.CreateProfile(profile)

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

	user, err := h.repo.GetUserByUsername(req.Username)
	if err != nil {
		api.Unauthorized(c, "用户名或密码错误")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		api.Unauthorized(c, "用户名或密码错误")
		return
	}

	// Check status
	if user.Status != 1 {
		api.Forbidden(c, "账号已被禁用")
		return
	}

	// TODO: Generate JWT token
	// For now, return user info
	user.Password = ""

	api.Success(c, gin.H{
		"user":  user,
		"token": "jwt-token-placeholder",
	})
}

// Logout handles user logout
func (h *Handler) Logout(c *gin.Context) {
	// TODO: Invalidate JWT token
	api.SuccessWithMessage(c, "退出成功", nil)
}

// ListUsers lists users with pagination
func (h *Handler) ListUsers(c *gin.Context) {
	var params api.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		params = api.DefaultPagination()
	}

	users, total, err := h.repo.ListUsers(params.GetOffset(), params.PerPage)
	if err != nil {
		api.InternalError(c, "获取用户列表失败")
		return
	}

	// Hide passwords
	for _, u := range users {
		u.Password = ""
	}

	api.Paginated(c, users, params.Page, params.PerPage, total)
}

// GetCurrentUser retrieves the current authenticated user
func (h *Handler) GetCurrentUser(c *gin.Context) {
	// TODO: Get user ID from JWT token
	userID := uint(1) // Placeholder

	user, err := h.repo.GetUserByID(userID)
	if err != nil {
		api.NotFound(c, "用户不存在")
		return
	}

	user.Password = ""
	api.Success(c, user)
}

// GetUser retrieves a user by ID
func (h *Handler) GetUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	user, err := h.repo.GetUserByID(uint(id))
	if err != nil {
		api.NotFound(c, "用户不存在")
		return
	}

	user.Password = ""
	api.Success(c, user)
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Nickname string `json:"nickname" binding:"omitempty,max=50"`
	Avatar   string `json:"avatar" binding:"omitempty,max=255"`
	Status   *int   `json:"status" binding:"omitempty,min=0,max=2"`
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

	user, err := h.repo.GetUserByID(uint(id))
	if err != nil {
		api.NotFound(c, "用户不存在")
		return
	}

	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}
	if req.Status != nil {
		user.Status = *req.Status
	}

	if err := h.repo.UpdateUser(user); err != nil {
		api.InternalError(c, "更新用户失败")
		return
	}

	user.Password = ""
	api.SuccessWithMessage(c, "用户更新成功", user)
}

// DeleteUser deletes a user
func (h *Handler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	if err := h.repo.DeleteUser(uint(id)); err != nil {
		api.InternalError(c, "删除用户失败")
		return
	}

	api.SuccessWithMessage(c, "用户删除成功", nil)
}

// GetProfile retrieves a user's profile
func (h *Handler) GetProfile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	profile, err := h.repo.GetProfileByUserID(uint(id))
	if err != nil {
		api.NotFound(c, "用户资料不存在")
		return
	}

	api.Success(c, profile)
}

// UpdateProfileRequest represents the request to update a profile
type UpdateProfileRequest struct {
	Gender   string     `json:"gender" binding:"omitempty,max=10"`
	Birthday *string   `json:"birthday" binding:"omitempty"`
	Location string     `json:"location" binding:"omitempty,max=100"`
	Bio      string     `json:"bio" binding:"omitempty,max=500"`
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

	profile, err := h.repo.GetProfileByUserID(uint(id))
	if err != nil {
		// Create profile if not exists
		profile = &UserProfile{UserID: uint(id)}
	}

	if req.Gender != "" {
		profile.Gender = req.Gender
	}
	if req.Location != "" {
		profile.Location = req.Location
	}
	if req.Bio != "" {
		profile.Bio = req.Bio
	}

	if err := h.repo.UpdateProfile(profile); err != nil {
		api.InternalError(c, "更新资料失败")
		return
	}

	api.SuccessWithMessage(c, "资料更新成功", profile)
}

// AddFriendRequest represents the request to add a friend
type AddFriendRequest struct {
	FriendID uint   `json:"friend_id" binding:"required"`
	Remark   string `json:"remark" binding:"max=50"`
}

// AddFriend adds a friend
func (h *Handler) AddFriend(c *gin.Context) {
	// TODO: Get user ID from JWT token
	userID := uint(1) // Placeholder

	var req AddFriendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if req.FriendID == userID {
		api.BadRequest(c, "不能添加自己为好友")
		return
	}

	friend := &Friend{
		UserID: userID,
		FriendID: req.FriendID,
		Status:  "pending",
		Remark:  req.Remark,
	}

	if err := h.repo.CreateFriend(friend); err != nil {
		api.InternalError(c, "添加好友失败")
		return
	}

	api.SuccessWithMessage(c, "好友请求已发送", friend)
}

// GetFriends retrieves friends for the current user
func (h *Handler) GetFriends(c *gin.Context) {
	// TODO: Get user ID from JWT token
	userID := uint(1) // Placeholder

	status := c.Query("status")

	friends, err := h.repo.GetFriends(userID, status)
	if err != nil {
		api.InternalError(c, "获取好友列表失败")
		return
	}

	api.Success(c, friends)
}

// UpdateFriendRequest represents the request to update a friend
type UpdateFriendRequest struct {
	Status string `json:"status" binding:"required,oneof=accepted blocked"`
	Remark string `json:"remark" binding:"max=50"`
}

// UpdateFriend updates a friend relationship
func (h *Handler) UpdateFriend(c *gin.Context) {
	friendID, err := strconv.ParseUint(c.Param("friend_id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的好友ID")
		return
	}

	// TODO: Get user ID from JWT token
	userID := uint(1) // Placeholder

	var req UpdateFriendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	friend := &Friend{
		UserID:   userID,
		FriendID: uint(friendID),
		Status:   req.Status,
		Remark:   req.Remark,
	}

	if err := h.repo.UpdateFriend(friend); err != nil {
		api.InternalError(c, "更新好友关系失败")
		return
	}

	api.SuccessWithMessage(c, "好友关系更新成功", friend)
}

// DeleteFriend deletes a friend
func (h *Handler) DeleteFriend(c *gin.Context) {
	friendID, err := strconv.ParseUint(c.Param("friend_id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的好友ID")
		return
	}

	// TODO: Get user ID from JWT token
	userID := uint(1) // Placeholder

	if err := h.repo.DeleteFriend(userID, uint(friendID)); err != nil {
		api.InternalError(c, "删除好友失败")
		return
	}

	api.SuccessWithMessage(c, "好友删除成功", nil)
}
