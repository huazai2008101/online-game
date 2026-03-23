package payment

import (
	"github.com/gin-gonic/gin"
	"online-game/pkg/api"
)

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	order := r.Group("/orders")
	{
		order.POST("", h.CreateOrder)
		order.GET("/:id", h.GetOrder)
		order.GET("", h.ListOrders)
	}

	score := r.Group("/scores")
	{
		score.GET("/:user_id", h.GetScore)
		score.POST("/recharge", h.Recharge)
		score.POST("/consume", h.Consume)
		score.GET("/:user_id/logs", h.GetScoreLogs)
	}
}

type CreateOrderRequest struct {
	UserID        uint   `json:"user_id" binding:"required"`
	ProductType   string `json:"product_type" binding:"required"`
	ProductID     string `json:"product_id" binding:"required"`
	Amount        int64  `json:"amount" binding:"required,min=1"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}

func (h *Handler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.ValidationError(c, err)
		return
	}

	order := &Order{
		UserID:        req.UserID,
		ProductType:   req.ProductType,
		ProductID:     req.ProductID,
		Amount:        req.Amount,
		PaymentMethod: req.PaymentMethod,
		Status:        "pending",
	}

	if err := h.repo.CreateOrder(order); err != nil {
		api.InternalError(c, "创建订单失败")
		return
	}

	api.SuccessWithMessage(c, "订单创建成功", order)
}

func (h *Handler) GetOrder(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetOrder"})
}

func (h *Handler) ListOrders(c *gin.Context) {
	api.Success(c, gin.H{"message": "ListOrders"})
}

func (h *Handler) GetScore(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetScore"})
}

func (h *Handler) Recharge(c *gin.Context) {
	api.Success(c, gin.H{"message": "Recharge"})
}

func (h *Handler) Consume(c *gin.Context) {
	api.Success(c, gin.H{"message": "Consume"})
}

func (h *Handler) GetScoreLogs(c *gin.Context) {
	api.Success(c, gin.H{"message": "GetScoreLogs"})
}
