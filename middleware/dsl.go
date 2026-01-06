package middleware

import (
	"api-gateway/config"
	"api-gateway/pkg/cos"
	"api-gateway/pkg/logger"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type DSLMiddleware struct {
	handlers  map[string]DSLHandler
	config    *config.Config
	cosClient *cos.Client
}

type DSLHandler interface {
	Handle(c *gin.Context) error
	Name() string
}

func NewDSLMiddleware(cfg *config.Config) *DSLMiddleware {
	m := &DSLMiddleware{
		handlers: make(map[string]DSLHandler),
		config:   cfg,
	}

	cosClient, err := cos.NewClient(&cfg.COS)
	if err != nil {
		logger.Errorf("Failed to create COS client: %v", err)
	} else {
		m.cosClient = cosClient
	}

	m.RegisterHandler(&URLReplaceHandler{cosClient: m.cosClient})

	return m
}

func (m *DSLMiddleware) RegisterHandler(handler DSLHandler) {
	m.handlers[handler.Name()] = handler
}

func (m *DSLMiddleware) Process() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := strings.TrimPrefix(c.Request.URL.Path, "/api")

		var dslList []string
		for _, target := range m.config.Targets {
			if target.Path != "" && target.Path == requestPath {
				dslList = target.PreHandlerDSL
				break
			}
		}

		if len(dslList) == 0 {
			c.Next()
			return
		}

		for _, dslName := range dslList {
			handler, exists := m.handlers[dslName]
			if !exists {
				logger.Infof("DSL handler not found: %s", dslName)
				continue
			}

			if err := handler.Handle(c); err != nil {
				logger.Errorf("DSL handler %s failed: %v", dslName, err)
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    40001,
					"message": err.Error(),
				})
				c.Abort()
				return
			}
		}

		logger.Infof("DSL middleware processing completed for path: %s", requestPath)
		c.Next()
	}
}

type URLReplaceHandler struct {
	cosClient *cos.Client
}

func (h *URLReplaceHandler) Name() string {
	return "url_replace"
}

func (h *URLReplaceHandler) Handle(c *gin.Context) error {
	var bodyBytes []byte
	if c.Request.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err != nil {
			return fmt.Errorf("读取请求体失败: %w", err)
		}
	}

	if len(bodyBytes) == 0 {
		return nil
	}

	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		logger.Infof("Body is not JSON format, skipping URL replacement")
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return nil
	}

	imageURL, exists := data["image_url"]
	if !exists {
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return nil
	}
	imageURLStr, _ := imageURL.(string)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	newURL, err := h.cosClient.UploadFromURL(ctx, imageURLStr, h.Name())
	if err != nil {
		logger.Errorf("Failed to upload image to COS: %v, original URL: %s", err, imageURLStr)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		return nil
	}

	logger.Infof("Successfully replaced URL: %s -> %s", imageURLStr, newURL)

	data["image_url"] = newURL

	modifiedBody, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("编码JSON失败: %w", err)
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(modifiedBody))

	return nil
}
