package payment

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"online-game/pkg/api"
)

// Handler handles HTTP requests for the payment service
type Handler struct {
	service *Service
}

// NewHandler creates a new handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all routes for the payment service
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	order := r.Group("/orders")
	{
		order.POST("", h.CreateOrder)
		order.GET("/:id", h.GetOrder)
		order.GET("/no/:order_no", h.GetOrderByNo)
		order.GET("", h.ListOrders)
		order.POST("/:id/pay", h.ProcessPayment)
		order.POST("/:id/refund", h.RefundOrder)
	}

	score := r.Group("/scores")
	{
		score.GET("/user/:user_id", h.GetScore)
		score.POST("/recharge", h.Recharge)
		score.POST("/consume", h.Consume)
		score.POST("/transfer", h.TransferScore)
		score.GET("/user/:user_id/logs", h.GetScoreLogs)
	}
}

// CreateOrder creates a new payment order
func (h *Handler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	order, err := h.service.CreateOrder(c.Request.Context(), &req)
	if err != nil {
		if err == ErrInvalidAmount {
			api.BadRequest(c, "金额无效")
		} else {
			api.InternalError(c, "创建订单失败")
		}
		return
	}

	api.SuccessWithMessage(c, "订单创建成功", order)
}

// GetOrder retrieves an order by ID
func (h *Handler) GetOrder(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的订单ID")
		return
	}

	order, err := h.service.GetOrder(c.Request.Context(), uint(id))
	if err != nil {
		api.NotFound(c, "订单不存在")
		return
	}

	api.Success(c, order)
}

// GetOrderByNo retrieves an order by order number
func (h *Handler) GetOrderByNo(c *gin.Context) {
	orderNo := c.Param("order_no")
	if orderNo == "" {
		api.BadRequest(c, "订单号不能为空")
		return
	}

	order, err := h.service.GetOrderByNo(c.Request.Context(), orderNo)
	if err != nil {
		api.NotFound(c, "订单不存在")
		return
	}

	api.Success(c, order)
}

// ListOrders lists orders with pagination
func (h *Handler) ListOrders(c *gin.Context) {
	var params api.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		params = api.DefaultPagination()
	}

	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 32)

	orders, total, err := h.service.ListOrders(c.Request.Context(), uint(userID), params.Page, params.PerPage)
	if err != nil {
		api.InternalError(c, "获取订单列表失败")
		return
	}

	api.Paginated(c, orders, params.Page, params.PerPage, total)
}

// ProcessPaymentRequest represents the request to process a payment
type ProcessPaymentRequest struct {
	PaymentData map[string]interface{} `json:"payment_data"`
}

// ProcessPayment processes a payment for an order
func (h *Handler) ProcessPayment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的订单ID")
		return
	}

	var req ProcessPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Empty request is fine
		req.PaymentData = make(map[string]interface{})
	}

	if err := h.service.ProcessPayment(c.Request.Context(), uint(id), req.PaymentData); err != nil {
		if err == ErrOrderNotFound {
			api.NotFound(c, "订单不存在")
		} else if err == ErrOrderAlreadyPaid {
			api.BadRequest(c, "订单已支付")
		} else {
			api.InternalError(c, "支付处理失败")
		}
		return
	}

	api.SuccessWithMessage(c, "支付成功", nil)
}

// RefundOrderRequest represents the request to refund an order
type RefundOrderRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// RefundOrder processes a refund for an order
func (h *Handler) RefundOrder(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的订单ID")
		return
	}

	var req RefundOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if err := h.service.RefundOrder(c.Request.Context(), uint(id), req.Reason); err != nil {
		if err == ErrOrderNotFound {
			api.NotFound(c, "订单不存在")
		} else {
			api.InternalError(c, "退款处理失败")
		}
		return
	}

	api.SuccessWithMessage(c, "退款成功", nil)
}

// GetScore retrieves user's score balance
func (h *Handler) GetScore(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	score, err := h.service.GetScore(c.Request.Context(), uint(userID))
	if err != nil {
		api.InternalError(c, "获取积分失败")
		return
	}

	api.Success(c, score)
}

// Recharge recharges score to user's account
func (h *Handler) Recharge(c *gin.Context) {
	var req RechargeScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	score, err := h.service.RechargeScore(c.Request.Context(), req.UserID, req.Amount, req.OrderID, req.Reason)
	if err != nil {
		if err == ErrInvalidAmount {
			api.BadRequest(c, "金额无效")
		} else {
			api.InternalError(c, "充值失败")
		}
		return
	}

	api.SuccessWithMessage(c, "充值成功", score)
}

// Consume consumes score from user's account
func (h *Handler) Consume(c *gin.Context) {
	var req ConsumeScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	score, err := h.service.ConsumeScore(c.Request.Context(), req.UserID, req.Amount, req.OrderID, req.Reason)
	if err != nil {
		if err == ErrInvalidAmount {
			api.BadRequest(c, "金额无效")
		} else if err == ErrInsufficientScore {
			api.BadRequest(c, "积分不足")
		} else {
			api.InternalError(c, "消费失败")
		}
		return
	}

	api.SuccessWithMessage(c, "消费成功", score)
}

// TransferScoreRequest represents the request to transfer score
type TransferScoreRequest struct {
	FromUserID uint   `json:"from_user_id" binding:"required"`
	ToUserID   uint   `json:"to_user_id" binding:"required"`
	Amount     int64  `json:"amount" binding:"required,min=1"`
	Reason     string `json:"reason"`
}

// TransferScore transfers score between users
func (h *Handler) TransferScore(c *gin.Context) {
	var req TransferScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	if req.FromUserID == req.ToUserID {
		api.BadRequest(c, "不能转账给自己")
		return
	}

	if err := h.service.TransferScore(c.Request.Context(), req.FromUserID, req.ToUserID, req.Amount, req.Reason); err != nil {
		if err == ErrInvalidAmount {
			api.BadRequest(c, "金额无效")
		} else if err == ErrInsufficientScore {
			api.BadRequest(c, "积分不足")
		} else {
			api.InternalError(c, "转账失败")
		}
		return
	}

	api.SuccessWithMessage(c, "转账成功", nil)
}

// GetScoreLogs retrieves score transaction logs
func (h *Handler) GetScoreLogs(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		api.BadRequest(c, "无效的用户ID")
		return
	}

	var params api.PaginationParams
	if err := c.ShouldBindQuery(&params); err != nil {
		params = api.DefaultPagination()
	}

	logs, total, err := h.service.GetScoreLogs(c.Request.Context(), uint(userID), params.Page, params.PerPage)
	if err != nil {
		api.InternalError(c, "获取积分记录失败")
		return
	}

	api.Paginated(c, logs, params.Page, params.PerPage, total)
}
