package repository

import (
	"api-gateway/model"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CallLogMongoRepository implements CallLogRepository using MongoDB
type CallLogMongoRepository struct {
	collection *mongo.Collection
}

// NewCallLogMongoRepository creates a new MongoDB call log repository
func NewCallLogMongoRepository(collection *mongo.Collection) CallLogRepository {
	return &CallLogMongoRepository{
		collection: collection,
	}
}

// Create creates a new call log entry
func (r *CallLogMongoRepository) Create(ctx context.Context, log *model.CallLog) error {
	if log.ID.IsZero() {
		log.ID = primitive.NewObjectID()
	}

	log.CreatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, log)
	if err != nil {
		return fmt.Errorf("failed to create call log: %w", err)
	}

	return nil
}

// GetByClientID retrieves call logs for a specific client
func (r *CallLogMongoRepository) GetByClientID(ctx context.Context, clientID primitive.ObjectID, offset, limit int) ([]*model.CallLog, error) {
	filter := bson.M{"client_id": clientID}

	opts := options.Find()
	opts.SetSkip(int64(offset))
	opts.SetLimit(int64(limit))
	opts.SetSort(bson.D{{Key: "created_at", Value: -1}}) // Sort by created_at descending

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get call logs by client ID: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*model.CallLog
	for cursor.Next(ctx) {
		var log model.CallLog
		if err := cursor.Decode(&log); err != nil {
			return nil, fmt.Errorf("failed to decode call log: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return logs, nil
}

// GetByAPIKey retrieves call logs for a specific API key
func (r *CallLogMongoRepository) GetByAPIKey(ctx context.Context, apiKey string, offset, limit int) ([]*model.CallLog, error) {
	filter := bson.M{"api_key": apiKey}

	opts := options.Find()
	opts.SetSkip(int64(offset))
	opts.SetLimit(int64(limit))
	opts.SetSort(bson.D{{Key: "created_at", Value: -1}}) // Sort by created_at descending

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get call logs by API key: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*model.CallLog
	for cursor.Next(ctx) {
		var log model.CallLog
		if err := cursor.Decode(&log); err != nil {
			return nil, fmt.Errorf("failed to decode call log: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return logs, nil
}

// List retrieves all call logs with pagination
func (r *CallLogMongoRepository) List(ctx context.Context, offset, limit int) ([]*model.CallLog, error) {
	opts := options.Find()
	opts.SetSkip(int64(offset))
	opts.SetLimit(int64(limit))
	opts.SetSort(bson.D{{Key: "created_at", Value: -1}}) // Sort by created_at descending

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list call logs: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*model.CallLog
	for cursor.Next(ctx) {
		var log model.CallLog
		if err := cursor.Decode(&log); err != nil {
			return nil, fmt.Errorf("failed to decode call log: %w", err)
		}
		logs = append(logs, &log)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return logs, nil
}
