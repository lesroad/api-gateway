package main

import (
	"api-gateway/config"
	"api-gateway/database"
	"api-gateway/pkg/logger"
	"api-gateway/router"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger.Init("api-gateway")
	defer logger.Close()

	cfg, err := config.NewConfig()
	if err != nil {
		logger.Errorf("Failed to load config: %v", err)
		os.Exit(1)
	}

	dbManager, err := database.NewDatabaseManager(cfg)
	if err != nil {
		logger.Errorf("Failed to initialize database: %v", err)
		os.Exit(1)
	}

	r := router.SetupRouter(dbManager.ClientRepo, dbManager.CallLogRepo)

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Infof("API Gateway starting on port %d", cfg.Port)
	logger.Infof("Config loaded - Database: %s, Targets: %v", cfg.Database.URL, cfg.Targets)

	go func() {
		if err := r.Run(addr); err != nil {
			logger.Errorf("Failed to start server: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := dbManager.Close(ctx); err != nil {
		logger.Errorf("Error closing database: %v", err)
	}

	logger.Info("Server exited")
}
