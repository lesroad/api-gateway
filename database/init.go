package database

import (
	"context"
	"fmt"

	"api-gateway/config"
	"api-gateway/pkg/logger"
	"api-gateway/repository"
	"api-gateway/service"
)

// DatabaseManager manages database connections and repositories
type DatabaseManager struct {
	MongoDB       *MongoDB
	ClientRepo    repository.ClientRepository
	CallLogRepo   repository.CallLogRepository
	ClientService *service.ClientService
}

// NewDatabaseManager creates a new database manager with all repositories
func NewDatabaseManager(cfg *config.Config) (*DatabaseManager, error) {
	// Connect to MongoDB
	logger.Infof("Connecting to MongoDB: %s/%s", cfg.Database.URL, cfg.Database.DB)
	mongoDB, err := NewMongoDB(cfg.Database.URL, cfg.Database.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	logger.Info("MongoDB connection established successfully")

	// Create repositories
	clientRepo := repository.NewClientMongoRepository(mongoDB.GetCollection("gw_clients"))
	callLogRepo := repository.NewCallLogMongoRepository(mongoDB.GetCollection("gw_call_logs"))

	// Create services
	clientService := service.NewClientService(clientRepo, callLogRepo)

	return &DatabaseManager{
		MongoDB:       mongoDB,
		ClientRepo:    clientRepo,
		CallLogRepo:   callLogRepo,
		ClientService: clientService,
	}, nil
}

// Close closes all database connections
func (dm *DatabaseManager) Close(ctx context.Context) error {
	logger.Info("Closing database connections...")
	err := dm.MongoDB.Close(ctx)
	if err != nil {
		logger.Errorf("Error closing MongoDB connection: %v", err)
	} else {
		logger.Info("Database connections closed successfully")
	}
	return err
}
