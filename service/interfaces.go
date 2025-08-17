package service

import (
	"api-gateway/model"
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ClientServiceInterface defines the interface for client service operations
type ClientServiceInterface interface {
	CreateClient(ctx context.Context, name, version string, initialCallCount int) (*model.Client, error)
	GetClientByAPIKey(ctx context.Context, apiKey string) (*model.Client, error)
	GetClientByID(ctx context.Context, id primitive.ObjectID) (*model.Client, error)
	ListClients(ctx context.Context, offset, limit int) ([]*model.Client, error)
	RechargeClient(ctx context.Context, id primitive.ObjectID, callCount int) error
	ConsumeCall(ctx context.Context, id primitive.ObjectID) error
	UpdateClientStatus(ctx context.Context, id primitive.ObjectID, status int) error
	DeleteClient(ctx context.Context, id primitive.ObjectID) error
	LogAPICall(ctx context.Context, clientID primitive.ObjectID, apiKey, version, path, method string, status int, duration int64) error
	GetClientCallLogs(ctx context.Context, clientID primitive.ObjectID, offset, limit int) ([]*model.CallLog, error)
}
