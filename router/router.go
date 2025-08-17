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

		api.Use(authMiddleware.Authenticate())
		api.Use(billingMiddleware.CheckCalls())
		api.Use(loggingMiddleware.LogAPICall())
		api.Use(billingMiddleware.DeductCalls())

		api.POST("/essay/evaluate/stream", proxyHandler.ProxyRequest)
		api.POST("/sts/ocr", proxyHandler.ProxyRequest)
	}

	admin := r.Group("/admin")
	{
		admin.POST("/clients", adminHandler.CreateClient)
		admin.GET("/clients", adminHandler.ListClients)
		admin.GET("/clients/:id", adminHandler.GetClient)
		admin.PUT("/clients/:id/status", adminHandler.UpdateClientStatus)

		admin.POST("/clients/:id/recharge", adminHandler.RechargeClient)

		admin.GET("/clients/:id/logs", adminHandler.GetClientCallLogs)
		admin.GET("/stats", adminHandler.GetStats)
	}

	return r
}
