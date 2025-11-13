package middleware

import (
	"api-gateway/config"
	"api-gateway/model"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/metrics"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type PrometheusMiddleware struct {
	metrics *metrics.APIGatewayMetrics
	config  *config.Config
}

func NewPrometheusMiddleware() *PrometheusMiddleware {
	return &PrometheusMiddleware{
		metrics: metrics.GetMetrics(),
		config:  config.GetConfig(),
	}
}

func (m *PrometheusMiddleware) Monitor() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// 获取客户端信息
		client := m.getClient(c)
		clientLabel := m.getClientLabel(client)

		// 记录请求大小
		requestSize := m.getRequestSize(c)
		if requestSize > 0 {
			m.metrics.RequestSize.WithLabelValues(clientLabel).Observe(float64(requestSize))
		}

		// 增加正在处理的请求数
		m.metrics.RequestsInFlight.WithLabelValues(clientLabel).Inc()

		// 使用自定义 ResponseWriter 来捕获响应大小
		writer := &responseWriter{
			ResponseWriter: c.Writer,
			statusCode:     200,
			bodySize:       0,
		}
		c.Writer = writer

		// 处理请求
		c.Next()

		// 处理完成，减少正在处理的请求数
		m.metrics.RequestsInFlight.WithLabelValues(clientLabel).Dec()

		// 计算请求延迟（毫秒）
		duration := time.Since(startTime).Milliseconds()
		statusCode := strconv.Itoa(writer.statusCode)

		// 记录请求总数
		m.metrics.RequestsTotal.WithLabelValues(clientLabel, statusCode).Inc()

		// 记录请求延迟
		m.metrics.RequestDuration.WithLabelValues(clientLabel).Observe(float64(duration))

		// 记录响应大小
		m.metrics.ResponseSize.WithLabelValues(clientLabel, statusCode).Observe(float64(writer.bodySize))

		// 检查是否超时（504 Gateway Timeout）
		if writer.statusCode == 504 {
			m.metrics.RequestTimeouts.WithLabelValues(clientLabel).Inc()
		}

		// 检查是否有错误（5xx）
		if writer.statusCode >= 500 {
			errorType := m.getErrorType(writer.statusCode)
			m.metrics.RequestErrors.WithLabelValues(clientLabel, errorType).Inc()
		}

		// 记录日志
		logger.Infof("Prometheus metrics recorded: client=%s, status=%d, duration=%dms, size=%d bytes",
			clientLabel, writer.statusCode, duration, writer.bodySize)
	}
}

// responseWriter 自定义 ResponseWriter，用于捕获响应大小和状态码
type responseWriter struct {
	gin.ResponseWriter
	statusCode int
	bodySize   int
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bodySize += n
	return n, err
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.bodySize += n
	return n, err
}

func (m *PrometheusMiddleware) getClient(c *gin.Context) *model.Client {
	clientInterface, exists := c.Get("client")
	if !exists {
		return nil
	}

	client, ok := clientInterface.(*model.Client)
	if !ok {
		return nil
	}

	return client
}

func (m *PrometheusMiddleware) getClientLabel(client *model.Client) string {
	if client == nil {
		return "unknown-unknown"
	}
	return fmt.Sprintf("%s-%s", client.Name, client.Version)
}

func (m *PrometheusMiddleware) getRequestSize(c *gin.Context) int64 {
	if c.Request.ContentLength > 0 {
		return c.Request.ContentLength
	}
	return 0
}

func (m *PrometheusMiddleware) getErrorType(statusCode int) string {
	switch statusCode {
	case 500:
		return "internal_error"
	case 502:
		return "bad_gateway"
	case 503:
		return "service_unavailable"
	case 504:
		return "gateway_timeout"
	default:
		if statusCode >= 500 && statusCode < 600 {
			return fmt.Sprintf("http_%d", statusCode)
		}
		return "unknown"
	}
}
