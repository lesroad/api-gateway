package main

import (
	"api-gateway/config"
	"api-gateway/database"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/queue"
	"api-gateway/pkg/worker"
	"api-gateway/repository"
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

	// 初始化任务存储库
	taskRepo := repository.NewTaskMongoRepository(dbManager.MongoDB.Database)

	var taskQueue queue.TaskQueue
	var workerPool *worker.WorkerPool

	if cfg.Async.Enabled {
		workerCount := cfg.Async.WorkerCount
		if workerCount == 0 {
			workerCount = 10 // 默认 10 个 worker
		}

		var err error
		taskQueue, err = queue.NewRedisTaskQueue(
			cfg.Async.Redis.Addr,
			cfg.Async.Redis.Password,
			cfg.Async.Redis.DB,
			cfg.Async.Redis.QueueKey,
		)
		if err != nil {
			logger.Errorf("Failed to connect to Redis: %v", err)
			os.Exit(1)
		}
		logger.Info("Redis queue initialized successfully")

		workerPool = worker.NewWorkerPool(workerCount, taskQueue, taskRepo)
		workerPool.Start()
	}

	r := router.SetupRouter(dbManager.ClientRepo, dbManager.CallLogRepo, taskRepo, taskQueue)

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

	// 停止 Worker Pool
	if workerPool != nil {
		workerPool.Stop()
	}

	// 关闭任务队列
	if taskQueue != nil {
		taskQueue.Close()
	}

	if err := dbManager.Close(ctx); err != nil {
		logger.Errorf("Error closing database: %v", err)
	}

	logger.Info("Server exited")
}
