package middleware

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"api-gateway/model"
	"api-gateway/pkg/logger"
	"api-gateway/repository"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LoggingMiddleware 调用日志记录中间件
type LoggingMiddleware struct {
	callLogRepo repository.CallLogRepository
}

// NewLoggingMiddleware 创建调用日志记录中间件
func NewLoggingMiddleware(callLogRepo repository.CallLogRepository) *LoggingMiddleware {
	return &LoggingMiddleware{
		callLogRepo: callLogRepo,
	}
}

// responseWriter 包装gin.ResponseWriter以捕获响应状态码
type responseWriter struct {
	gin.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.body != nil {
		rw.body.Write(b)
	}
	return rw.ResponseWriter.Write(b)
}

// LogAPICall 记录API调用日志
func (l *LoggingMiddleware) LogAPICall() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录开始时间
		startTime := time.Now()

		// 从上下文中获取客户信息（由认证中间件设置）
		clientInterface, exists := c.Get("client")
		if !exists {
			// 如果没有客户信息，说明认证失败，仍然记录日志但使用空值
			c.Next()
			return
		}

		client, ok := clientInterface.(*model.Client)
		if !ok {
			c.Next()
			return
		}

		// 包装ResponseWriter以捕获状态码
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			statusCode:     http.StatusOK, // 默认状态码
			body:           bytes.NewBuffer(nil),
		}
		c.Writer = rw

		// 处理请求
		c.Next()

		// 计算响应时间
		duration := time.Since(startTime).Milliseconds()

		// 创建调用日志
		callLog := model.NewCallLog(
			client.ID,
			client.APIKey,
			extractVersionFromPath(c.Request.URL.Path),
			c.Request.URL.Path,
			c.Request.Method,
			rw.statusCode,
			duration,
		)

		// 异步记录日志，避免影响响应性能
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := l.callLogRepo.Create(ctx, callLog); err != nil {
				// 记录日志失败不应该影响主要业务流程
				logger.Errorf("Failed to create call log: %v", err)
			} else {
				logger.Infof("API call logged: %s %s by client %s, status: %d, duration: %dms",
					callLog.Method, callLog.Path, callLog.ClientID.Hex(), callLog.Status, callLog.Duration)
			}
		}()
	}
}

// LogFailedRequest 记录失败的请求（认证失败、计费失败等）
func (l *LoggingMiddleware) LogFailedRequest(clientID primitive.ObjectID, apiKey, reason string) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// 包装ResponseWriter以捕获状态码
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			statusCode:     http.StatusOK,
			body:           bytes.NewBuffer(nil),
		}
		c.Writer = rw

		// 处理请求
		c.Next()

		// 计算响应时间
		duration := time.Since(startTime).Milliseconds()

		// 创建调用日志
		callLog := model.NewCallLog(
			clientID,
			apiKey,
			extractVersionFromPath(c.Request.URL.Path),
			c.Request.URL.Path,
			c.Request.Method,
			rw.statusCode,
			duration,
		)

		// 异步记录日志
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := l.callLogRepo.Create(ctx, callLog); err != nil {
				logger.Errorf("Failed to create failed request log: %v", err)
			} else {
				logger.Infof("Failed request logged: %s %s by client %s, reason: %s, status: %d",
					callLog.Method, callLog.Path, callLog.ClientID.Hex(), reason, callLog.Status)
			}
		}()
	}
}

// extractVersionFromPath 从请求路径中提取版本信息
func extractVersionFromPath(path string) string {
	// 从路径中提取版本，例如 /api/v1/essay/evaluate/stream -> v1
	if len(path) >= 7 && path[:7] == "/api/v1" {
		return "v1"
	}
	if len(path) >= 7 && path[:7] == "/api/v2" {
		return "v2"
	}
	return "unknown"
}
