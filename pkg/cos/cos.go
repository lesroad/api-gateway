package cos

import (
	"api-gateway/config"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

type Client struct {
	cosClient *cos.Client
	baseURL   string
}

func NewClient(cfg *config.COSConfig) (*Client, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("COS SecretID and SecretKey are required")
	}

	bucketURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid COS base URL: %w", err)
	}

	cosClient := cos.NewClient(&cos.BaseURL{
		BucketURL: bucketURL,
	}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
		Timeout: 30 * time.Second,
	})

	return &Client{
		cosClient: cosClient,
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
	}, nil
}

func (c *Client) UploadFromURL(ctx context.Context, imageURL, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download image, status code: %d", resp.StatusCode)
	}

	objectKey := c.generateObjectKey(imageURL, name)

	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: c.getContentType(objectKey),
		},
	}

	_, err = c.cosClient.Object.Put(ctx, objectKey, resp.Body, opt)
	if err != nil {
		return "", fmt.Errorf("failed to upload to COS: %w", err)
	}

	newURL := fmt.Sprintf("%s/%s", c.baseURL, objectKey)
	return newURL, nil
}

func (c *Client) generateObjectKey(imageURL, name string) string {
	urlPath := imageURL
	if idx := strings.Index(urlPath, "?"); idx != -1 {
		urlPath = urlPath[:idx]
	}

	// 取后8位字符
	suffix := ""
	if len(urlPath) >= 8 {
		suffix = urlPath[len(urlPath)-8:]
	} else {
		suffix = urlPath
	}

	// 格式: name/timestamp_suffix.jpg
	return fmt.Sprintf("%s/%s_%s.jpg", name, time.Now().Format("20060102150405"), suffix)
}

func (c *Client) getContentType(objectKey string) string {
	ext := strings.ToLower(path.Ext(objectKey))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}
