package signature

import (
	"api-gateway/pkg/logger"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 根据 https://open.xkw.com/document/guide-signature 实现
type XKWSignatureGenerator struct {
	AppId     string
	AppSecret string
}

func NewXKWSignatureGenerator(appId, appSecret string) *XKWSignatureGenerator {
	return &XKWSignatureGenerator{
		AppId:     appId,
		AppSecret: appSecret,
	}
}

func (g *XKWSignatureGenerator) GetType() string {
	return TypeXKW
}

func (g *XKWSignatureGenerator) GenerateHeaders(method, path string, body []byte, params map[string]string) (map[string]string, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := generateNonce(32)
	if err != nil {
		return nil, fmt.Errorf("生成随机串失败: %w", err)
	}
	signMap := make(map[string]string)

	xopBody := string(body)
	if len(xopBody) > 0 {
		encodedBody := encodeSpecialChars(xopBody) // 特殊字符编码
		signMap["xop_body"] = encodedBody
	}

	signMap["Xop-App-Id"] = g.AppId
	signMap["Xop-Timestamp"] = timestamp
	signMap["Xop-Nonce"] = nonce
	signMap["xop_url"] = strings.TrimPrefix(path, "/api") // （不含域名的路径）

	sortedKeys := make([]string, 0, len(signMap))
	for k := range signMap {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var buf bytes.Buffer
	for i, k := range sortedKeys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(signMap[k])
	}

	buf.WriteString("&secret=")
	buf.WriteString(g.AppSecret)
	normalizedStr := buf.String()
	logger.Infof("normalizedStr:%s", normalizedStr)

	base64Str := base64.StdEncoding.EncodeToString([]byte(normalizedStr))

	sha1Hash := sha1.Sum([]byte(base64Str))

	signature := hex.EncodeToString(sha1Hash[:])

	headers := map[string]string{
		"Xop-App-Id":    g.AppId,
		"Xop-Timestamp": timestamp,
		"Xop-Nonce":     nonce,
		"Xop-Sign":      signature,
	}

	return headers, nil
}

func generateNonce(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("随机串长度必须大于0")
	}

	const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	buf := make([]byte, length)
	for i := range buf {
		buf[i] = charset[r.Intn(len(charset))]
	}
	return string(buf), nil
}

func encodeSpecialChars(s string) string {
	// 替换规则：参考文档中的特殊字符编码表
	replaceMap := map[string]string{
		"&": "\\u0026",
		"'": "\\u0027",
		"<": "\\u003c",
		"=": "\\u003d",
		">": "\\u003e",
	}

	result := s
	for old, new := range replaceMap {
		result = strings.ReplaceAll(result, old, new)
	}
	return result
}
