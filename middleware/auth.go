package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"api-gateway/config"
	"api-gateway/errors"
	"api-gateway/pkg/logger"
	"api-gateway/repository"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 认证中间件
type AuthMiddleware struct {
	clientRepo         repository.ClientRepository
	signatureValidator SignatureValidator
	config             *config.Config
}

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(clientRepo repository.ClientRepository, signatureValidator SignatureValidator, cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{
		clientRepo:         clientRepo,
		signatureValidator: signatureValidator,
		config:             cfg,
	}
}

// Authenticate 认证中间件处理函数
func (a *AuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 提取API密钥
		apiKey := a.extractAPIKey(c)
		if apiKey == "" {
			errors.RespondWithError(c, http.StatusUnauthorized, errors.NewInvalidAPIKeyError())
			return
		}

		// 创建超时上下文
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		// 根据API密钥查找客户
		client, err := a.clientRepo.GetByAPIKey(ctx, apiKey)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				logger.Infof("Authentication failed: invalid API key %s", apiKey)
				errors.RespondWithError(c, http.StatusUnauthorized, errors.NewInvalidAPIKeyError())
				return
			}
			// 数据库错误，返回内部服务器错误
			logger.Errorf("Database error during authentication: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "内部服务器错误",
			})
			c.Abort()
			return
		}

		// 检查客户状态
		if !client.IsActive() {
			logger.Infof("Authentication failed: client %s is disabled", client.ID.Hex())
			errors.RespondWithError(c, http.StatusForbidden, errors.NewClientDisabledError(client.ID.Hex()))
			return
		}

		// 签名验证（如果启用）
		if a.config.Auth.EnableSignature {
			if err := a.signatureValidator.ValidateSignature(c.Request, client); err != nil {
				logger.Infof("Signature validation failed for client %s: %v", client.ID.Hex(), err)
				a.handleSignatureError(c, err)
				return
			}
			logger.Infof("Signature validation successful for client %s", client.ID.Hex())
		}

		// 将客户信息存储到上下文中，供后续中间件使用
		c.Set("client", client)
		c.Set("api_key", apiKey)

		logger.Infof("Authentication successful for client %s (%s)", client.ID.Hex(), client.Name)
		c.Next()
	}
}

// extractAPIKey 从请求中提取API密钥
func (a *AuthMiddleware) extractAPIKey(c *gin.Context) string {
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		return apiKey
	}
	return ""
}

// handleSignatureError 处理签名验证错误
func (a *AuthMiddleware) handleSignatureError(c *gin.Context, err error) {
	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "missing signature"):
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    40101,
			"message": "Signature validation failed",
			"error":   "missing signature",
		})
	case strings.Contains(errMsg, "missing timestamp"):
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    40102,
			"message": "Signature validation failed",
			"error":   "missing timestamp",
		})
	case strings.Contains(errMsg, "invalid timestamp"):
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    40103,
			"message": "Signature validation failed",
			"error":   "invalid timestamp format",
		})
	case strings.Contains(errMsg, "timestamp expired"):
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    40104,
			"message": "Signature validation failed",
			"error":   "timestamp expired",
		})
	case strings.Contains(errMsg, "invalid signature"):
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    40105,
			"message": "Signature validation failed",
			"error":   "invalid signature",
		})
	default:
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    40100,
			"message": "Signature validation failed",
			"error":   "signature validation error",
		})
	}
	c.Abort()
}
