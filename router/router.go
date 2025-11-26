package router

import (
	"api-gateway/config"
	"api-gateway/handler"
	"api-gateway/middleware"
	"api-gateway/pkg/metrics"
	"api-gateway/pkg/queue"
	"api-gateway/repository"
	"api-gateway/service"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRouter(clientRepo repository.ClientRepository, callLogRepo repository.CallLogRepository,
	taskRepo repository.TaskRepository, taskQueue queue.TaskQueue) *gin.Engine {
	gin.SetMode(gin.ReleaseMode) // 设置为 release 模式
	r := gin.New()               // 不添加任何中间件
	r.Use(gin.Recovery())

	cfg := config.GetConfig()

	metrics.InitMetrics()

	timeWindow := time.Duration(cfg.Auth.SignatureTimeWindow) * time.Second
	signatureValidator := middleware.NewHMACSignatureValidator(timeWindow)

	authMiddleware := middleware.NewAuthMiddleware(clientRepo, signatureValidator, cfg)
	rateLimitMiddleware := middleware.NewRateLimitMiddleware()
	billingMiddleware := middleware.NewBillingMiddleware(clientRepo, callLogRepo)
	loggingMiddleware := middleware.NewLoggingMiddleware(callLogRepo)
	prometheusMiddleware := middleware.NewPrometheusMiddleware()

	// 只在启用异步功能时创建异步中间件
	var asyncMiddleware *middleware.AsyncMiddleware
	if taskQueue != nil {
		asyncMiddleware = middleware.NewAsyncMiddleware(taskQueue, taskRepo, cfg)
	}

	clientService := service.NewClientService(clientRepo, callLogRepo)

	proxyHandler := handler.NewProxyHandler()
	adminHandler := handler.NewAdminHandler(clientService)
	taskHandler := handler.NewTaskHandler(taskRepo)

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 测试接口
	r.POST("/test/callback", proxyHandler.CallbackHandler)

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":  "ok",
				"service": "api-gateway",
			})
		})

		api.Use(authMiddleware.Authenticate())   // 1. 认证
		api.Use(rateLimitMiddleware.RateLimit()) // 2. 限流
		api.Use(billingMiddleware.CheckCalls())  // 3. 检查次数
		api.Use(billingMiddleware.DeductCalls()) // 4. 扣减次数（异步请求也要先扣费）
		api.Use(loggingMiddleware.LogAPICall())  // 5. 记录日志
		if asyncMiddleware != nil {
			api.Use(asyncMiddleware.HandleAsync()) // 6. 异步处理（异步请求在这里提前返回）
		}
		api.Use(prometheusMiddleware.Monitor()) // 7. Prometheus 监控

		// 业务接口
		api.POST("/essay/evaluate/stream", proxyHandler.ProxyRequest)
		api.POST("/sts/ocr", proxyHandler.ProxyRequest)
		api.POST("/essay/evaluate/english", proxyHandler.ProxyRequest)
		api.POST("/math/quesiton_similar_recommend", proxyHandler.ProxyRequest)
		api.POST("/math/process", proxyHandler.ProxyRequest)
		api.POST("/math/cropping", proxyHandler.ProxyRequest)
		api.POST("/essay/statistics", proxyHandler.ProxyRequest)

		// 任务查询接口
		api.GET("/tasks/:task_id", taskHandler.GetTask)
		api.GET("/tasks/:task_id/status", taskHandler.GetTaskStatus)
		api.GET("/tasks", taskHandler.ListTasks)
	}

	admin := r.Group("/admin")
	{
		admin.POST("/clients", adminHandler.CreateClient)
		admin.GET("/clients", adminHandler.ListClients)
		admin.GET("/clients/:id", adminHandler.GetClient)
		admin.PUT("/clients/:id/status", adminHandler.UpdateClientStatus)
		admin.PUT("/clients/:id/qps", adminHandler.UpdateClientQPS)

		admin.POST("/clients/:id/recharge", adminHandler.RechargeClient)

		admin.GET("/clients/:id/logs", adminHandler.GetClientCallLogs)
		admin.GET("/stats", adminHandler.GetStats)
	}

	return r
}
