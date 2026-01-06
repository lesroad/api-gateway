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
	dslMiddleware := middleware.NewDSLMiddleware(cfg)
	loggingMiddleware := middleware.NewLoggingMiddleware(callLogRepo)
	prometheusMiddleware := middleware.NewPrometheusMiddleware()
	asyncMiddleware := middleware.NewAsyncMiddleware(taskQueue, taskRepo, cfg)

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
		api.Use(dslMiddleware.Process())         // 5. DSL规则引擎处理
		api.Use(loggingMiddleware.LogAPICall())  // 6. 记录日志
		if asyncMiddleware != nil {
			api.Use(asyncMiddleware.HandleAsync()) // 7. 异步处理（异步请求在这里提前返回）
		}
		api.Use(prometheusMiddleware.Monitor()) // 8. Prometheus 监控

		// 从配置文件动态注册业务接口
		registerRoutesFromConfig(api, proxyHandler, cfg)

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

func registerRoutesFromConfig(api *gin.RouterGroup, proxyHandler *handler.ProxyHandler, cfg *config.Config) {
	for _, target := range cfg.Targets {
		if target.Path != "" {
			api.POST(target.Path, proxyHandler.ProxyRequest)
		}
	}
}
