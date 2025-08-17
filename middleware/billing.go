package middleware

import (
	"context"
	"net/http"
	"time"

	"api-gateway/errors"
	"api-gateway/model"
	"api-gateway/pkg/logger"
	"api-gateway/repository"

	"github.com/gin-gonic/gin"
)

// BillingMiddleware 计费中间件
type BillingMiddleware struct {
	clientRepo repository.ClientRepository
	logRepo    repository.CallLogRepository
}

// NewBillingMiddleware 创建计费中间件
func NewBillingMiddleware(clientRepo repository.ClientRepository, logRepo repository.CallLogRepository) *BillingMiddleware {
	return &BillingMiddleware{
		clientRepo: clientRepo,
		logRepo:    logRepo,
	}
}

// CheckCalls 检查调用次数（不扣减）
func (b *BillingMiddleware) CheckCalls() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文中获取客户信息（由认证中间件设置）
		clientInterface, exists := c.Get("client")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "内部服务器错误：客户信息未找到",
			})
			c.Abort()
			return
		}

		client, ok := clientInterface.(*model.Client)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "内部服务器错误：客户信息类型错误",
			})
			c.Abort()
			return
		}

		// 检查剩余调用次数
		if !client.HasCallsRemaining() {
			logger.Infof("Billing check failed: client %s has insufficient calls (remaining: %d)",
				client.ID.Hex(), client.CallCount)
			errors.RespondWithError(c, http.StatusPaymentRequired,
				errors.NewInsufficientCallsError(client.CallCount, client.ID.Hex()))
			return
		}

		// 记录调用开始时间，用于后续日志记录
		c.Set("billing_start_time", time.Now())
		c.Set("billing_checked", true) // 标记已检查过次数

		logger.Infof("Billing check passed: client %s has %d calls remaining",
			client.ID.Hex(), client.CallCount)
		c.Next()
	}
}

// DeductCalls 扣减调用次数（仅在响应成功时调用）
func (b *BillingMiddleware) DeductCalls() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // 先执行后续处理器

		// 只有在响应状态码为200时才扣减次数
		if c.Writer.Status() != http.StatusOK {
			logger.Infof("Request failed with status %d, skipping billing deduction", c.Writer.Status())
			return
		}

		// 检查是否已经检查过次数
		if checked, exists := c.Get("billing_checked"); !exists || !checked.(bool) {
			logger.Errorf("Billing deduction called without prior check")
			return
		}

		// 从上下文中获取客户信息
		clientInterface, exists := c.Get("client")
		if !exists {
			logger.Error("Client information not found during billing deduction")
			return
		}

		client, ok := clientInterface.(*model.Client)
		if !ok {
			logger.Error("Invalid client information type during billing deduction")
			return
		}

		// 创建超时上下文
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 原子性地扣减调用次数
		err := b.clientRepo.DeductCallCount(ctx, client.ID)
		if err != nil {
			// 扣减失败，记录错误但不影响响应（因为请求已经成功）
			logger.Errorf("Failed to deduct call count for client %s: %v", client.ID.Hex(), err)
			return
		}

		logger.Infof("Billing deduction successful: deducted 1 call from client %s", client.ID.Hex())
	}
}

// CheckAndDeduct 检查并扣减调用次数（保留原方法以兼容性）
func (b *BillingMiddleware) CheckAndDeduct() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文中获取客户信息（由认证中间件设置）
		clientInterface, exists := c.Get("client")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "内部服务器错误：客户信息未找到",
			})
			c.Abort()
			return
		}

		client, ok := clientInterface.(*model.Client)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "内部服务器错误：客户信息类型错误",
			})
			c.Abort()
			return
		}

		// 检查剩余调用次数
		if !client.HasCallsRemaining() {
			logger.Infof("Billing check failed: client %s has insufficient calls (remaining: %d)",
				client.ID.Hex(), client.CallCount)
			errors.RespondWithError(c, http.StatusPaymentRequired,
				errors.NewInsufficientCallsError(client.CallCount, client.ID.Hex()))
			return
		}

		// 创建超时上下文
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		// 原子性地扣减调用次数
		err := b.clientRepo.DeductCallCount(ctx, client.ID)
		if err != nil {
			// 如果扣减失败，检查是否是因为余额不足
			if err.Error() == "insufficient calls" {
				logger.Infof("Billing deduction failed: client %s has insufficient calls", client.ID.Hex())
				errors.RespondWithError(c, http.StatusPaymentRequired,
					errors.NewInsufficientCallsError(0, client.ID.Hex()))
				return
			}

			// 其他数据库错误
			logger.Errorf("Database error during billing deduction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "内部服务器错误：扣费失败",
			})
			c.Abort()
			return
		}

		// 更新上下文中的客户信息（减少一次调用）
		client.DecrementCallCount()
		c.Set("client", client)

		// 记录调用开始时间，用于后续日志记录
		c.Set("billing_start_time", time.Now())

		logger.Infof("Billing successful: deducted 1 call from client %s (remaining: %d)",
			client.ID.Hex(), client.CallCount)
		c.Next()
	}
}
