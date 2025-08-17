package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"api-gateway/model"
	"api-gateway/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ClientService provides business logic for client operations
type ClientService struct {
	clientRepo  repository.ClientRepository
	callLogRepo repository.CallLogRepository
}

// NewClientService creates a new client service
func NewClientService(clientRepo repository.ClientRepository, callLogRepo repository.CallLogRepository) *ClientService {
	return &ClientService{
		clientRepo:  clientRepo,
		callLogRepo: callLogRepo,
	}
}

// CreateClient creates a new client with a generated API key and secret
func (s *ClientService) CreateClient(ctx context.Context, name, version string, initialCallCount int) (*model.Client, error) {
	// Generate API key
	apiKey, err := s.generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Generate secret
	secret, err := s.generateSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}

	// Check if API key already exists (very unlikely but good to check)
	existing, _ := s.clientRepo.GetByAPIKey(ctx, apiKey)
	if existing != nil {
		// Regenerate if collision occurs
		apiKey, err = s.generateAPIKey()
		if err != nil {
			return nil, fmt.Errorf("failed to regenerate API key: %w", err)
		}
	}

	client := model.NewClient(name, apiKey, secret, version, initialCallCount)

	err = s.clientRepo.Create(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return client, nil
}

// GetClientByAPIKey retrieves a client by API key
func (s *ClientService) GetClientByAPIKey(ctx context.Context, apiKey string) (*model.Client, error) {
	return s.clientRepo.GetByAPIKey(ctx, apiKey)
}

// GetClientByID retrieves a client by ID
func (s *ClientService) GetClientByID(ctx context.Context, id primitive.ObjectID) (*model.Client, error) {
	return s.clientRepo.GetByID(ctx, id)
}

// ListClients retrieves all clients with pagination
func (s *ClientService) ListClients(ctx context.Context, offset, limit int) ([]*model.Client, error) {
	return s.clientRepo.List(ctx, offset, limit)
}

// RechargeClient adds call count to a client
func (s *ClientService) RechargeClient(ctx context.Context, id primitive.ObjectID, callCount int) error {
	if callCount <= 0 {
		return fmt.Errorf("call count must be positive")
	}

	return s.clientRepo.UpdateCallCount(ctx, id, callCount)
}

// ConsumeCall decrements the call count for a client
func (s *ClientService) ConsumeCall(ctx context.Context, id primitive.ObjectID) error {
	return s.clientRepo.UpdateCallCount(ctx, id, -1)
}

// UpdateClientStatus updates the status of a client
func (s *ClientService) UpdateClientStatus(ctx context.Context, id primitive.ObjectID, status int) error {
	client, err := s.clientRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	client.Status = status
	return s.clientRepo.Update(ctx, client)
}

// DeleteClient deletes a client
func (s *ClientService) DeleteClient(ctx context.Context, id primitive.ObjectID) error {
	return s.clientRepo.Delete(ctx, id)
}

// LogAPICall logs an API call
func (s *ClientService) LogAPICall(ctx context.Context, clientID primitive.ObjectID, apiKey, version, path, method string, status int, duration int64) error {
	log := model.NewCallLog(clientID, apiKey, version, path, method, status, duration)
	return s.callLogRepo.Create(ctx, log)
}

// GetClientCallLogs retrieves call logs for a client
func (s *ClientService) GetClientCallLogs(ctx context.Context, clientID primitive.ObjectID, offset, limit int) ([]*model.CallLog, error) {
	return s.callLogRepo.GetByClientID(ctx, clientID, offset, limit)
}

// generateAPIKey generates a random API key
func (s *ClientService) generateAPIKey() (string, error) {
	bytes := make([]byte, 32) // 64 character hex string
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return "ak_" + hex.EncodeToString(bytes), nil
}

// generateSecret generates a random secret for HMAC signing
func (s *ClientService) generateSecret() (string, error) {
	bytes := make([]byte, 32) // 64 character hex string
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
