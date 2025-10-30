package router

import (
	"api-gateway/config"
	"api-gateway/handler"
	"api-gateway/middleware"
	"api-gateway/repository"
	"api-gateway/service"
	"time"

	"github.com/gin-gonic/gin"
)

func SetupRouter(clientRepo repository.ClientRepository, callLogRepo repository.CallLogRepository) *gin.Engine {
	r := gin.Default()

	cfg := config.GetConfig()

	timeWindow := time.Duration(cfg.Auth.SignatureTimeWindow) * time.Second
	signatureValidator := middleware.NewHMACSignatureValidator(timeWindow)

	authMiddleware := middleware.NewAuthMiddleware(clientRepo, signatureValidator, cfg)
	rateLimitMiddleware := middleware.NewRateLimitMiddleware()
	billingMiddleware := middleware.NewBillingMiddleware(clientRepo, callLogRepo)
	loggingMiddleware := middleware.NewLoggingMiddleware(callLogRepo)

	clientService := service.NewClientService(clientRepo, callLogRepo)

	proxyHandler := handler.NewProxyHandler()
	adminHandler := handler.NewAdminHandler(clientService)

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
		api.Use(loggingMiddleware.LogAPICall())  // 4. 记录日志
		api.Use(billingMiddleware.DeductCalls()) // 5. 扣减次数

		api.POST("/essay/evaluate/stream", proxyHandler.ProxyRequest)
		api.POST("/sts/ocr", proxyHandler.ProxyRequest)
		api.POST("/essay/evaluate/english", proxyHandler.ProxyRequest)
		api.POST("/math/quesiton_similar_recommend", proxyHandler.ProxyRequest)
		api.POST("/math/process", proxyHandler.ProxyRequest)
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
