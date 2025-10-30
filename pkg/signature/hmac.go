package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"
)

type HMACSignatureGenerator struct {
	AccessKey string
	SecretKey string
}

func NewHMACSignatureGenerator(accessKey, secretKey string) *HMACSignatureGenerator {
	return &HMACSignatureGenerator{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}
}

func (g *HMACSignatureGenerator) GetType() string {
	return TypeHMAC
}

func (g *HMACSignatureGenerator) GenerateHeaders(method, path string, body []byte, params map[string]string) (map[string]string, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	// 计算请求体哈希
	bodyHash := sha256.Sum256(body)
	bodyHashStr := fmt.Sprintf("%x", bodyHash)

	// 构建签名字符串: HTTP方法 + "\n" + URI路径 + "\n" + 时间戳 + "\n" + 请求体哈希
	message := fmt.Sprintf("%s\n%s\n%s\n%s", method, path, timestamp, bodyHashStr)

	// 使用HMAC-SHA256生成签名
	h := hmac.New(sha256.New, []byte(g.SecretKey))
	h.Write([]byte(message))

	// Base64编码输出
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 返回需要添加到header的参数
	headers := map[string]string{
		"Authorization": fmt.Sprintf("HMAC-SHA256 Credential=%s, Signature=%s", g.AccessKey, signature),
		"X-Timestamp":   timestamp,
	}

	return headers, nil
}
