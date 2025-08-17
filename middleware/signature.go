package middleware

import (
	"api-gateway/model"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// SignatureValidator 签名验证器接口
type SignatureValidator interface {
	ValidateSignature(req *http.Request, client *model.Client) error
	GenerateSignature(method, path, timestamp, bodyHash, secret string) string
}

// HMACSignatureValidator HMAC签名验证器实现
type HMACSignatureValidator struct {
	timeWindow time.Duration // 时间窗口，默认5分钟
}

// NewHMACSignatureValidator 创建HMAC签名验证器
func NewHMACSignatureValidator(timeWindow time.Duration) *HMACSignatureValidator {
	if timeWindow == 0 {
		timeWindow = 5 * time.Minute // 默认5分钟时间窗口
	}
	return &HMACSignatureValidator{
		timeWindow: timeWindow,
	}
}

// ValidateSignature 验证请求签名
func (v *HMACSignatureValidator) ValidateSignature(req *http.Request, client *model.Client) error {
	// 1. 提取请求头中的签名和时间戳
	signature := req.Header.Get("X-Signature")
	if signature == "" {
		return fmt.Errorf("missing signature")
	}

	timestamp := req.Header.Get("X-Timestamp")
	if timestamp == "" {
		return fmt.Errorf("missing timestamp")
	}

	// 2. 验证时间戳
	if err := v.validateTimestamp(timestamp); err != nil {
		return err
	}

	// 3. 计算请求体哈希
	bodyHash, err := v.calculateBodyHash(req)
	if err != nil {
		return fmt.Errorf("failed to calculate body hash: %w", err)
	}

	// 4. 生成期望的签名
	expectedSignature := v.GenerateSignature(req.Method, req.URL.Path, timestamp, bodyHash, client.Secret)

	// 5. 比较签名
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// GenerateSignature 生成HMAC-SHA256签名
func (v *HMACSignatureValidator) GenerateSignature(method, path, timestamp, bodyHash, secret string) string {
	// 构建签名字符串: HTTP方法 + "\n" + URI路径 + "\n" + 时间戳 + "\n" + 请求体哈希
	message := fmt.Sprintf("%s\n%s\n%s\n%s", method, path, timestamp, bodyHash)

	// 使用HMAC-SHA256生成签名
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))

	// Base64编码输出
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// validateTimestamp 验证时间戳
func (v *HMACSignatureValidator) validateTimestamp(timestampStr string) error {
	// 解析时间戳
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format")
	}

	// 获取当前时间
	now := time.Now().Unix()
	requestTime := timestamp

	// 检查时间差是否在允许的窗口内
	timeDiff := now - requestTime
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	if time.Duration(timeDiff)*time.Second > v.timeWindow {
		return fmt.Errorf("timestamp expired")
	}

	return nil
}

// calculateBodyHash 计算请求体的SHA256哈希
func (v *HMACSignatureValidator) calculateBodyHash(req *http.Request) (string, error) {
	if req.Body == nil {
		// 空请求体的哈希
		return fmt.Sprintf("%x", sha256.Sum256([]byte{})), nil
	}

	// 读取请求体
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return "", err
	}

	// 重新设置请求体，以便后续处理可以再次读取
	req.Body = io.NopCloser(bytes.NewReader(body))

	// 计算SHA256哈希
	hash := sha256.Sum256(body)
	return fmt.Sprintf("%x", hash), nil
}
