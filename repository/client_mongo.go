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

// ClientMongoRepository implements ClientRepository using MongoDB
type ClientMongoRepository struct {
	collection *mongo.Collection
}

// NewClientMongoRepository creates a new MongoDB client repository
func NewClientMongoRepository(collection *mongo.Collection) ClientRepository {
	return &ClientMongoRepository{
		collection: collection,
	}
}

// Create creates a new client
func (r *ClientMongoRepository) Create(ctx context.Context, client *model.Client) error {
	if client.ID.IsZero() {
		client.ID = primitive.NewObjectID()
	}

	client.CreatedAt = time.Now()
	client.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	return nil
}

// GetByID retrieves a client by ID
func (r *ClientMongoRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*model.Client, error) {
	var client model.Client

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&client)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("client not found")
		}
		return nil, fmt.Errorf("failed to get client by ID: %w", err)
	}

	return &client, nil
}

// GetByAPIKey retrieves a client by API key
func (r *ClientMongoRepository) GetByAPIKey(ctx context.Context, apiKey string) (*model.Client, error) {
	var client model.Client

	err := r.collection.FindOne(ctx, bson.M{"api_key": apiKey}).Decode(&client)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("client not found")
		}
		return nil, fmt.Errorf("failed to get client by API key: %w", err)
	}

	return &client, nil
}

// UpdateCallCount updates the call count for a client
func (r *ClientMongoRepository) UpdateCallCount(ctx context.Context, id primitive.ObjectID, delta int) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$inc": bson.M{"call_count": delta},
		"$set": bson.M{"updated_at": time.Now()},
	}

	// If delta is positive, also update total_count
	if delta > 0 {
		update["$inc"].(bson.M)["total_count"] = delta
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update call count: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}

// Update updates a client
func (r *ClientMongoRepository) Update(ctx context.Context, client *model.Client) error {
	client.UpdatedAt = time.Now()

	filter := bson.M{"_id": client.ID}
	update := bson.M{"$set": client}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update client: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}

// List retrieves all clients with pagination
func (r *ClientMongoRepository) List(ctx context.Context, offset, limit int) ([]*model.Client, error) {
	opts := options.Find()
	opts.SetSkip(int64(offset))
	opts.SetLimit(int64(limit))
	opts.SetSort(bson.D{{Key: "created_at", Value: -1}}) // Sort by created_at descending

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list clients: %w", err)
	}
	defer cursor.Close(ctx)

	var clients []*model.Client
	for cursor.Next(ctx) {
		var client model.Client
		if err := cursor.Decode(&client); err != nil {
			return nil, fmt.Errorf("failed to decode client: %w", err)
		}
		clients = append(clients, &client)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return clients, nil
}

// DeductCallCount atomically decrements call count by 1, returns error if insufficient calls
func (r *ClientMongoRepository) DeductCallCount(ctx context.Context, id primitive.ObjectID) error {
	// 使用findOneAndUpdate进行原子操作
	filter := bson.M{
		"_id":        id,
		"call_count": bson.M{"$gt": 0}, // 只有当call_count > 0时才更新
	}
	update := bson.M{
		"$inc": bson.M{"call_count": -1},
		"$set": bson.M{"updated_at": time.Now()},
	}

	var result model.Client
	err := r.collection.FindOneAndUpdate(ctx, filter, update).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// 没有找到符合条件的文档，说明余额不足或客户不存在
			return fmt.Errorf("insufficient calls")
		}
		return fmt.Errorf("failed to deduct call count: %w", err)
	}

	return nil
}

// UpdateQPS updates the QPS limit for a client
func (r *ClientMongoRepository) UpdateQPS(ctx context.Context, id primitive.ObjectID, qps int) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"qps":        qps,
			"updated_at": time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update client QPS: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}

// Delete deletes a client by ID
func (r *ClientMongoRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete client: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("client not found")
	}

	return nil
}
