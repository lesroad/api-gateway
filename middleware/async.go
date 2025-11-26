package middleware

import (
	"api-gateway/config"
	"api-gateway/model"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/queue"
	"api-gateway/repository"
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AsyncMiddleware struct {
	taskQueue queue.TaskQueue
	taskRepo  repository.TaskRepository
	config    *config.Config
}

func NewAsyncMiddleware(taskQueue queue.TaskQueue, taskRepo repository.TaskRepository, cfg *config.Config) *AsyncMiddleware {
	return &AsyncMiddleware{
		taskQueue: taskQueue,
		taskRepo:  taskRepo,
		config:    cfg,
	}
}

func (m *AsyncMiddleware) HandleAsync() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否需要异步处理
		isAsync := c.GetHeader("X-Async")
		callbackURL := c.GetHeader("X-Callback-URL")

		// 如果不是异步请求，继续正常处理
		if isAsync != "true" || callbackURL == "" {
			c.Next()
			return
		}

		// 获取客户端信息
		clientInterface, exists := c.Get("client")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "客户端信息未找到",
			})
			c.Abort()
			return
		}

		client, ok := clientInterface.(*model.Client)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "客户端信息类型错误",
			})
			c.Abort()
			return
		}

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    40000,
				"message": "读取请求体失败",
			})
			c.Abort()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		taskID := uuid.New().String()

		// 收集请求头（排除敏感信息）
		headers := make(map[string]string)
		for key, values := range c.Request.Header {
			if len(values) > 0 && !isSensitiveHeader(key) {
				headers[key] = values[0]
			}
		}

		// 获取回调相关的headers
		callbackHeaders := make(map[string]string)
		if authHeader := c.GetHeader("X-Callback-Auth"); authHeader != "" {
			callbackHeaders["Authorization"] = authHeader
		}

		// 获取目标URL
		var targetURLStr string
		if target, exists := m.config.Targets[client.Version]; exists {
			targetURLStr = target.URL
		} else {
			logger.Errorf("Unsupported client version: %s", client.Version)
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    40000,
				"message": "不支持的客户端版本",
			})
			c.Abort()
			return
		}

		// 创建任务
		task := model.NewTask(
			taskID,
			client.ID.Hex(),
			client.APIKey,
			c.Request.Method,
			c.Request.URL.Path,
			targetURLStr,
			callbackURL,
			headers,
			string(bodyBytes),
		)

		// 设置回调方法
		if callbackMethod := c.GetHeader("X-Callback-Method"); callbackMethod != "" {
			task.CallbackMethod = callbackMethod
		}

		// 设置回调headers
		task.CallbackHeaders = callbackHeaders

		// 保存任务到数据库
		if err := m.taskRepo.Create(c.Request.Context(), task); err != nil {
			logger.Errorf("Failed to create task: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "创建任务失败",
			})
			c.Abort()
			return
		}

		// 将任务加入队列
		if err := m.taskQueue.Enqueue(task); err != nil {
			logger.Errorf("Failed to enqueue task: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    50300,
				"message": "任务队列已满，请稍后重试",
			})
			c.Abort()
			return
		}

		// 立即返回任务ID
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "任务已接受，将异步处理",
			"data": gin.H{
				"task_id":      taskID,
				"status":       model.TaskStatusPending,
				"callback_url": callbackURL,
				"created_at":   task.CreatedAt,
			},
		})

		// 阻止后续中间件执行
		c.Abort()
	}
}

func isSensitiveHeader(key string) bool {
	sensitiveHeaders := []string{
		"Authorization",
		"X-API-Key",
		"X-Signature",
		"X-Secret",
		"Cookie",
	}

	for _, h := range sensitiveHeaders {
		if key == h {
			return true
		}
	}
	return false
}
