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

type TaskRepository interface {
	Create(ctx context.Context, task *model.Task) error
	GetByTaskID(ctx context.Context, taskID string) (*model.Task, error)
	GetByID(ctx context.Context, id string) (*model.Task, error)
	Update(ctx context.Context, task *model.Task) error
	UpdateStatus(ctx context.Context, taskID string, status model.TaskStatus) error
	GetTasksByClient(ctx context.Context, clientID string, limit int, offset int) ([]*model.Task, error)
	IncrementCallbackAttempts(ctx context.Context, taskID string) error
	DeleteExpiredTasks(ctx context.Context) (int64, error)
}

type taskMongoRepository struct {
	collection *mongo.Collection
}

func NewTaskMongoRepository(db *mongo.Database) TaskRepository {
	collection := db.Collection("tasks")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// task_id 唯一索引
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "task_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	// client_id 索引（用于查询客户端的任务）
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "client_id", Value: 1}, {Key: "created_at", Value: -1}},
	})

	// status + created_at 索引（用于获取待处理任务）
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: 1}},
	})

	// expire_at TTL 索引（自动删除过期任务）
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expire_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	})

	return &taskMongoRepository{
		collection: collection,
	}
}

// Create 创建任务
func (r *taskMongoRepository) Create(ctx context.Context, task *model.Task) error {
	result, err := r.collection.InsertOne(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	task.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// GetByTaskID 通过任务ID获取任务
func (r *taskMongoRepository) GetByTaskID(ctx context.Context, taskID string) (*model.Task, error) {
	var task model.Task
	err := r.collection.FindOne(ctx, bson.M{"task_id": taskID}).Decode(&task)
	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("task not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return &task, nil
}

// GetByID 通过MongoDB ID获取任务
func (r *taskMongoRepository) GetByID(ctx context.Context, id string) (*model.Task, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid task id: %w", err)
	}

	var task model.Task
	err = r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&task)
	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("task not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return &task, nil
}

// Update 更新任务
func (r *taskMongoRepository) Update(ctx context.Context, task *model.Task) error {
	filter := bson.M{"task_id": task.TaskID}
	_, err := r.collection.ReplaceOne(ctx, filter, task)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}
	return nil
}

// UpdateStatus 更新任务状态
func (r *taskMongoRepository) UpdateStatus(ctx context.Context, taskID string, status model.TaskStatus) error {
	filter := bson.M{"task_id": taskID}
	update := bson.M{"$set": bson.M{"status": status}}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	return nil
}

// GetTasksByClient 获取客户端的任务列表
func (r *taskMongoRepository) GetTasksByClient(ctx context.Context, clientID string, limit int, offset int) ([]*model.Task, error) {
	filter := bson.M{"client_id": clientID}
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(offset)).
		SetSort(bson.D{{Key: "created_at", Value: -1}}) // 最新的在前

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks by client: %w", err)
	}
	defer cursor.Close(ctx)

	var tasks []*model.Task
	if err = cursor.All(ctx, &tasks); err != nil {
		return nil, fmt.Errorf("failed to decode tasks: %w", err)
	}

	return tasks, nil
}

// IncrementCallbackAttempts 增加回调尝试次数
func (r *taskMongoRepository) IncrementCallbackAttempts(ctx context.Context, taskID string) error {
	filter := bson.M{"task_id": taskID}
	update := bson.M{
		"$inc": bson.M{"callback_attempts": 1},
		"$set": bson.M{"last_callback_at": time.Now()},
	}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to increment callback attempts: %w", err)
	}
	return nil
}

// DeleteExpiredTasks 删除过期的任务（备用，通常由 TTL 索引自动处理）
func (r *taskMongoRepository) DeleteExpiredTasks(ctx context.Context) (int64, error) {
	filter := bson.M{"expire_at": bson.M{"$lt": time.Now()}}
	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired tasks: %w", err)
	}
	return result.DeletedCount, nil
}
