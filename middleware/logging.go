package middleware

import (
	"api-gateway/errors"
	"api-gateway/model"
	"api-gateway/pkg/logger"
	"api-gateway/repository"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

// loggingResponseWriter 包装gin.ResponseWriter以捕获响应状态码和内容
// 避免与标准库的ResponseWriter混淆，使用更具体的命名
type loggingResponseWriter struct {
	gin.ResponseWriter
	statusCode        int
	notStreamResponse *bytes.Buffer
	isStreamRequest   bool
	streamBuffer      *bytes.Buffer // 收集所有流式数据
}

// newLoggingResponseWriter 创建一个新的日志记录响应写入器
func newLoggingResponseWriter(w gin.ResponseWriter, isStream bool) *loggingResponseWriter {
	return &loggingResponseWriter{
		ResponseWriter:    w,
		statusCode:        http.StatusOK, // 默认状态码
		notStreamResponse: bytes.NewBuffer(nil),
		isStreamRequest:   isStream,
		streamBuffer:      bytes.NewBuffer(nil),
	}
}

// CaptureStatusCode 捕获HTTP状态码
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// CaptureResponseData 捕获响应数据并写入到底层ResponseWriter
func (lrw *loggingResponseWriter) Write(data []byte) (int, error) {
	// 如果是流式请求，先收集数据，不立即解析
	if lrw.isStreamRequest {
		lrw.streamBuffer.Write(data)
	} else {
		lrw.notStreamResponse.Write(data)
	}

	// 写入到实际的ResponseWriter
	return lrw.ResponseWriter.Write(data)
}

// GetStatusCode 获取HTTP状态码
func (lrw *loggingResponseWriter) GetStatusCode() int {
	return lrw.statusCode
}

// GetResponseBody 获取响应体内容
func (lrw *loggingResponseWriter) GetResponseBody() string {
	if lrw.isStreamRequest {
		// 对于流式响应，现在解析完整数据并提取最终结果
		return lrw.parseCompleteStreamData()
	}

	// 对于普通响应，返回完整的响应体
	return lrw.notStreamResponse.String()
}

// parseCompleteStreamData 解析完整的流式数据，提取最终的完成消息
func (lrw *loggingResponseWriter) parseCompleteStreamData() string {
	content := lrw.streamBuffer.String()

	// 解析SSE格式的流式数据
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 查找以 "data: " 开头的行
		if strings.HasPrefix(line, "data: ") {
			dataContent := strings.TrimPrefix(line, "data: ")

			// 尝试解析JSON数据
			var messageData map[string]any
			if err := json.Unmarshal([]byte(dataContent), &messageData); err != nil {
				continue // 跳过无效的JSON
			}

			// 每次找到complete或error消息都更新，最后一个会被保留
			if msgType, exists := messageData["type"].(string); exists && (msgType == "complete" || msgType == "error") {
				return dataContent
			}
		}
	}

	return ""
}

// LogAPICall 记录API调用日志
func (l *LoggingMiddleware) LogAPICall() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录开始时间
		startTime := time.Now()

		// 从上下文中获取客户信息
		clientInterface, exists := c.Get("client")
		if !exists {
			errors.RespondWithError(c, http.StatusUnauthorized, errors.NewAPIError(errors.ErrInvalidAPIKey, "认证失败", nil))
			return
		}

		client, ok := clientInterface.(*model.Client)
		if !ok {
			errors.RespondWithError(c, http.StatusUnauthorized, errors.NewAPIError(errors.ErrInvalidAPIKey, "认证失败", nil))
			return
		}

		// 读取请求体
		requestBody := ""
		if c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				requestBody = string(bodyBytes)
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// 检查是否为流式响应
		isStream := strings.Contains(c.Request.URL.Path, "/stream")

		// 创建日志记录响应写入器
		lrw := newLoggingResponseWriter(c.Writer, isStream)
		c.Writer = lrw

		// 处理请求
		c.Next()

		// 计算响应时间
		duration := time.Since(startTime).Milliseconds()

		// 获取响应内容
		responseBody := lrw.GetResponseBody()

		// 创建调用日志
		callLog := model.NewCallLogWithParams(
			client.ID,
			client.APIKey,
			c.Request.URL.Path,
			lrw.GetStatusCode(),
			duration,
			requestBody,
			responseBody,
		)

		// 异步记录日志，避免影响响应性能
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			if err := l.callLogRepo.Create(ctx, callLog); err != nil {
				// 记录日志失败不应该影响主要业务流程
				logger.Errorf("Failed to create call log: %v", err)
			} else {
				logger.Infof("API call logged: %s by client %s, status: %d, duration: %dms",
					callLog.Path, callLog.ClientID.Hex(), callLog.Status, callLog.Duration)
			}
		}()
	}
}
