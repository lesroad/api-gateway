package queue

import (
	"api-gateway/model"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisTaskQueue struct {
	client    *redis.Client
	queueKey  string
	ctx       context.Context
	blockTime time.Duration
}

func NewRedisTaskQueue(redisAddr, password string, db int, queueKey string) (*RedisTaskQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     password,
		DB:           db,
		PoolSize:     100,
		MinIdleConns: 10,
		MaxRetries:   3,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	if queueKey == "" {
		queueKey = "api_gateway:task_queue"
	}

	return &RedisTaskQueue{
		client:    client,
		queueKey:  queueKey,
		ctx:       ctx,
		blockTime: 5 * time.Second, // BRPOP 阻塞时间
	}, nil
}

func (q *RedisTaskQueue) Enqueue(task *model.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	if err := q.client.LPush(q.ctx, q.queueKey, data).Err(); err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	return nil
}

func (q *RedisTaskQueue) Dequeue(ctx context.Context) (*model.Task, error) {
	// BRPOP：阻塞式右侧弹出（先进先出）
	result, err := q.client.BRPop(ctx, q.blockTime, q.queueKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		if err == context.Canceled || err == context.DeadlineExceeded {
			return nil, err
		}
		return nil, fmt.Errorf("failed to dequeue task: %w", err)
	}

	if len(result) < 2 {
		return nil, fmt.Errorf("invalid BRPOP result")
	}

	var task model.Task
	if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &task, nil
}

func (q *RedisTaskQueue) Close() error {
	return q.client.Close()
}
