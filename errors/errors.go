package errors

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// Error codes as defined in the design document
const (
	// Authentication related errors
	ErrInvalidAPIKey  = 40001 // API密钥无效
	ErrClientDisabled = 40002 // 客户已禁用

	// Billing related errors
	ErrInsufficientCalls = 40301 // 调用次数不足
	ErrCallLimitExceeded = 42901 // 调用频率超限
	ErrRateLimitExceeded = 42902 // QPS限流超限

	// Proxy related errors
	ErrUpstreamTimeout = 50401 // 上游服务超时
	ErrUpstreamError   = 50402 // 上游服务错误

	// Version related errors
	ErrUnsupportedVersion = 40004 // 不支持的版本
)

// APIError represents an API error response
type APIError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return fmt.Sprintf("API Error %d: %s", e.Code, e.Message)
}

// NewAPIError creates a new API error
func NewAPIError(code int, message string, data interface{}) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// RespondWithError sends an error response
func RespondWithError(c *gin.Context, httpStatus int, apiError *APIError) {
	c.JSON(httpStatus, apiError)
	c.Abort()
}

// Authentication errors
func NewInvalidAPIKeyError() *APIError {
	return NewAPIError(ErrInvalidAPIKey, "API密钥无效", nil)
}

func NewClientDisabledError(clientID string) *APIError {
	return NewAPIError(ErrClientDisabled, "客户已禁用", gin.H{
		"client_id": clientID,
	})
}

// Billing errors
func NewInsufficientCallsError(remainingCalls int, clientID string) *APIError {
	return NewAPIError(ErrInsufficientCalls, "调用次数不足，请充值", gin.H{
		"remaining_calls": remainingCalls,
		"client_id":       clientID,
	})
}

// Version errors
func NewUnsupportedVersionError(version string) *APIError {
	return NewAPIError(ErrUnsupportedVersion, "不支持的版本", gin.H{
		"version": version,
	})
}

// Proxy errors
func NewUpstreamTimeoutError() *APIError {
	return NewAPIError(ErrUpstreamTimeout, "上游服务超时", nil)
}

func NewUpstreamError(message string) *APIError {
	return NewAPIError(ErrUpstreamError, "上游服务错误", gin.H{
		"upstream_message": message,
	})
}

// Rate limit errors
func NewRateLimitExceededError(clientID string, qps int) *APIError {
	return NewAPIError(ErrRateLimitExceeded, "请求频率超限，请稍后重试", gin.H{
		"client_id": clientID,
		"qps_limit": qps,
	})
}
