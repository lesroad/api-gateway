package repository

import (
	"context"

	"api-gateway/model"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ClientRepository defines the interface for client data operations
type ClientRepository interface {
	// Create creates a new client
	Create(ctx context.Context, client *model.Client) error

	// GetByID retrieves a client by ID
	GetByID(ctx context.Context, id primitive.ObjectID) (*model.Client, error)

	// GetByAPIKey retrieves a client by API key
	GetByAPIKey(ctx context.Context, apiKey string) (*model.Client, error)

	// UpdateCallCount updates the call count for a client
	UpdateCallCount(ctx context.Context, id primitive.ObjectID, delta int) error

	// DeductCallCount atomically decrements call count by 1, returns error if insufficient calls
	DeductCallCount(ctx context.Context, id primitive.ObjectID) error

	// Update updates a client
	Update(ctx context.Context, client *model.Client) error

	// List retrieves all clients with pagination
	List(ctx context.Context, offset, limit int) ([]*model.Client, error)

	// Delete deletes a client by ID
	Delete(ctx context.Context, id primitive.ObjectID) error
}

// CallLogRepository defines the interface for call log operations
type CallLogRepository interface {
	// Create creates a new call log entry
	Create(ctx context.Context, log *model.CallLog) error

	// GetByClientID retrieves call logs for a specific client
	GetByClientID(ctx context.Context, clientID primitive.ObjectID, offset, limit int) ([]*model.CallLog, error)

	// GetByAPIKey retrieves call logs for a specific API key
	GetByAPIKey(ctx context.Context, apiKey string, offset, limit int) ([]*model.CallLog, error)

	// List retrieves all call logs with pagination
	List(ctx context.Context, offset, limit int) ([]*model.CallLog, error)
}
