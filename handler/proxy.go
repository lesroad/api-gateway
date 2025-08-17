package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"api-gateway/config"
	"api-gateway/errors"
	"api-gateway/model"
	"api-gateway/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ProxyHandler 代理处理器
type ProxyHandler struct {
	client *http.Client
	config *config.Config
}

// NewProxyHandler 创建代理处理器
func NewProxyHandler() *ProxyHandler {
	return &ProxyHandler{
		client: &http.Client{
			Timeout: 0,
		},
		config: config.GetConfig(),
	}
}

// ProxyRequest 代理请求处理
func (p *ProxyHandler) ProxyRequest(c *gin.Context) {
	// 从上下文中获取客户信息
	clientInterface, exists := c.Get("client")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    50000,
			"message": "内部服务器错误：客户信息未找到",
		})
		return
	}

	client, ok := clientInterface.(*model.Client)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    50000,
			"message": "内部服务器错误：客户信息类型错误",
		})
		return
	}

	// 根据客户版本获取目标URL
	targetURL, err := p.getTargetURL(client.Version)
	if err != nil {
		logger.Errorf("Failed to get target URL for version %s: %v", client.Version, err)
		errors.RespondWithError(c, http.StatusBadRequest,
			errors.NewUnsupportedVersionError(client.Version))
		return
	}

	// 创建代理请求
	proxyReq, err := p.createProxyRequest(c, targetURL)
	if err != nil {
		logger.Errorf("Failed to create proxy request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    50000,
			"message": fmt.Sprintf("创建代理请求失败: %v", err),
		})
		return
	}

	// 设置超时
	timeout := p.getTimeoutForVersion(client.Version)
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	proxyReq = proxyReq.WithContext(ctx)

	// 发送请求
	logger.Infof("Proxying request to %s for client %s", targetURL, client.ID.Hex())
	resp, err := p.client.Do(proxyReq)
	if err != nil {
		logger.Errorf("Upstream request failed: %v", err)
		p.handleUpstreamError(c, err)
		return
	}
	defer resp.Body.Close()

	logger.Infof("Received response from upstream: status %d", resp.StatusCode)
	// 转发响应
	p.forwardResponse(c, resp)
}

// getTargetURL 根据版本获取目标URL
func (p *ProxyHandler) getTargetURL(version string) (string, error) {
	if p.config == nil {
		return "", fmt.Errorf("配置未初始化")
	}

	target, exists := p.config.Targets[version]
	if !exists {
		return "", fmt.Errorf("不支持的版本: %s", version)
	}

	return target.URL, nil
}

// getTimeoutForVersion 根据版本获取超时时间
func (p *ProxyHandler) getTimeoutForVersion(version string) time.Duration {
	if p.config == nil {
		return 30 * time.Second
	}

	target, exists := p.config.Targets[version]
	if !exists {
		return 30 * time.Second
	}

	return time.Duration(target.Timeout) * time.Millisecond
}

// createProxyRequest 创建代理请求
func (p *ProxyHandler) createProxyRequest(c *gin.Context, targetURL string) (*http.Request, error) {
	// 读取原始请求体
	var bodyBytes []byte
	if c.Request.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err != nil {
			return nil, fmt.Errorf("读取请求体失败: %w", err)
		}
		c.Request.Body.Close()
	}

	// 创建新的请求
	proxyReq, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 复制请求头，但跳过一些不应该转发的头
	skipHeaders := map[string]bool{
		"host":           true,
		"content-length": true,
		"x-api-key":      true, // API密钥不应该转发给上游服务
	}

	for name, values := range c.Request.Header {
		if !skipHeaders[strings.ToLower(name)] {
			for _, value := range values {
				proxyReq.Header.Add(name, value)
			}
		}
	}

	// 设置Content-Length
	if len(bodyBytes) > 0 {
		proxyReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
	}

	// 设置User-Agent
	proxyReq.Header.Set("User-Agent", "API-Gateway/1.0")

	return proxyReq, nil
}

// forwardResponse 转发响应
func (p *ProxyHandler) forwardResponse(c *gin.Context, resp *http.Response) {
	// 复制响应头，但跳过一些不应该转发的头
	skipHeaders := map[string]bool{
		"Content-Length":    true,
		"Transfer-Encoding": true,
		"Connection":        true,
	}

	for name, values := range resp.Header {
		if !skipHeaders[name] {
			for _, value := range values {
				c.Header(name, value)
			}
		}
	}

	// 设置状态码
	c.Status(resp.StatusCode)

	// 检查是否是流式响应
	if p.isStreamingResponse(resp) {
		p.forwardStreamingResponse(c, resp)
	} else {
		p.forwardRegularResponse(c, resp)
	}
}

// isStreamingResponse 检查是否是流式响应
func (p *ProxyHandler) isStreamingResponse(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	transferEncoding := resp.Header.Get("Transfer-Encoding")

	// 检查是否是Server-Sent Events或者chunked编码
	return strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "text/plain") ||
		strings.Contains(transferEncoding, "chunked")
}

// forwardStreamingResponse 转发流式响应
func (p *ProxyHandler) forwardStreamingResponse(c *gin.Context, resp *http.Response) {
	// 设置流式响应的必要头部
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// 获取ResponseWriter的Flusher接口
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		// 如果不支持流式响应，回退到普通响应
		p.forwardRegularResponse(c, resp)
		return
	}

	// 创建缓冲区用于读取数据
	buffer := make([]byte, 4096)

	for {
		// 从上游响应读取数据
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			// 写入到客户端
			if _, writeErr := c.Writer.Write(buffer[:n]); writeErr != nil {
				// 客户端连接断开
				break
			}
			// 立即刷新缓冲区
			flusher.Flush()
		}

		if err != nil {
			if err == io.EOF {
				// 正常结束
				logger.Info("Streaming response completed")
				break
			}
			// 其他错误，记录但不中断（可能是网络抖动）
			logger.Errorf("Error reading streaming response: %v", err)
			break
		}
	}
}

// forwardRegularResponse 转发普通响应
func (p *ProxyHandler) forwardRegularResponse(c *gin.Context, resp *http.Response) {
	// 直接复制响应体
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		// 如果复制失败，可能是客户端断开连接
		return
	}
}

// handleUpstreamError 处理上游服务错误
func (p *ProxyHandler) handleUpstreamError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	// 检查错误类型
	if strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded") {
		// 超时错误
		errors.RespondWithError(c, http.StatusGatewayTimeout,
			errors.NewUpstreamTimeoutError())
		return
	}

	if strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no such host") {
		// 连接错误
		errors.RespondWithError(c, http.StatusBadGateway,
			errors.NewUpstreamError("上游服务不可用"))
		return
	}

	// 其他错误
	errors.RespondWithError(c, http.StatusBadGateway,
		errors.NewUpstreamError(fmt.Sprintf("上游服务错误: %v", err)))
}
