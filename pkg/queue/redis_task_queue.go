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

// Enqueue 将任务加入队列（使用 LPUSH，左进右出）
func (q *RedisTaskQueue) Enqueue(task *model.Task) error {
	// 序列化任务
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// 推入队列（左侧）
	if err := q.client.LPush(q.ctx, q.queueKey, data).Err(); err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	return nil
}

// Dequeue 从队列中取出任务（使用 BRPOP，阻塞式右侧弹出）
func (q *RedisTaskQueue) Dequeue(ctx context.Context) (*model.Task, error) {
	// BRPOP：阻塞式右侧弹出（先进先出）
	result, err := q.client.BRPop(ctx, q.blockTime, q.queueKey).Result()
	if err != nil {
		if err == redis.Nil {
			// 队列为空，返回 nil（不是错误）
			return nil, nil
		}
		if err == context.Canceled || err == context.DeadlineExceeded {
			return nil, err
		}
		return nil, fmt.Errorf("failed to dequeue task: %w", err)
	}

	// result[0] 是 key，result[1] 是 value
	if len(result) < 2 {
		return nil, fmt.Errorf("invalid BRPOP result")
	}

	// 反序列化任务
	var task model.Task
	if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &task, nil
}

// Size 返回队列当前大小
func (q *RedisTaskQueue) Size() int {
	size, err := q.client.LLen(q.ctx, q.queueKey).Result()
	if err != nil {
		return 0
	}
	return int(size)
}

// Close 关闭 Redis 连接
func (q *RedisTaskQueue) Close() error {
	return q.client.Close()
}

// Clear 清空队列（谨慎使用！）
func (q *RedisTaskQueue) Clear() error {
	return q.client.Del(q.ctx, q.queueKey).Err()
}

// Peek 查看队列头部元素（不移除）
func (q *RedisTaskQueue) Peek() (*model.Task, error) {
	result, err := q.client.LIndex(q.ctx, q.queueKey, -1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to peek task: %w", err)
	}

	var task model.Task
	if err := json.Unmarshal([]byte(result), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &task, nil
}

// GetStats 获取队列统计信息
func (q *RedisTaskQueue) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["queue_size"] = q.Size()

	// 获取 Redis 连接池信息
	poolStats := q.client.PoolStats()
	stats["pool_hits"] = poolStats.Hits
	stats["pool_misses"] = poolStats.Misses
	stats["pool_timeouts"] = poolStats.Timeouts
	stats["total_conns"] = poolStats.TotalConns
	stats["idle_conns"] = poolStats.IdleConns

	return stats
}
