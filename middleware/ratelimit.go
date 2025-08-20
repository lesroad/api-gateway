package middleware

import (
	"api-gateway/errors"
	"api-gateway/model"
	"api-gateway/pkg/logger"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TokenBucket 令牌桶结构
type TokenBucket struct {
	capacity   int        // 桶容量
	tokens     int        // 当前令牌数
	refillRate int        // 每秒补充令牌数
	lastRefill time.Time  // 上次补充时间
	mutex      sync.Mutex // 互斥锁
}

// NewTokenBucket 创建新的令牌桶
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// TakeToken 尝试获取令牌
func (tb *TokenBucket) TakeToken() bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	// 补充令牌
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * tb.refillRate

	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = now
	}

	// 尝试获取令牌
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// RateLimitMiddleware 限流中间件
type RateLimitMiddleware struct {
	buckets map[string]*TokenBucket // 客户端ID -> 令牌桶
	mutex   sync.RWMutex            // 读写锁
}

// NewRateLimitMiddleware 创建限流中间件
func NewRateLimitMiddleware() *RateLimitMiddleware {
	rl := &RateLimitMiddleware{
		buckets: make(map[string]*TokenBucket),
	}

	// 启动清理协程，定期清理不活跃的令牌桶
	go rl.cleanup()

	return rl
}

// RateLimit 限流处理函数
func (rl *RateLimitMiddleware) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文中获取客户信息（由认证中间件设置）
		clientInterface, exists := c.Get("client")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "内部服务器错误：客户信息未找到",
			})
			c.Abort()
			return
		}

		client, ok := clientInterface.(*model.Client)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    50000,
				"message": "内部服务器错误：客户信息类型错误",
			})
			c.Abort()
			return
		}

		// 获取或创建客户的令牌桶
		bucket := rl.getOrCreateBucket(client.ID.Hex(), client.QPS)

		// 尝试获取令牌
		if !bucket.TakeToken() {
			logger.Infof("Rate limit exceeded for client %s (QPS: %d)", client.ID.Hex(), client.QPS)
			errors.RespondWithError(c, http.StatusTooManyRequests,
				errors.NewRateLimitExceededError(client.ID.Hex(), client.QPS))
			return
		}

		logger.Debugf("Rate limit check passed for client %s", client.ID.Hex())
		c.Next()
	}
}

// getOrCreateBucket 获取或创建令牌桶
func (rl *RateLimitMiddleware) getOrCreateBucket(clientID string, qps int) *TokenBucket {
	rl.mutex.RLock()
	bucket, exists := rl.buckets[clientID]
	rl.mutex.RUnlock()

	if exists {
		// 检查QPS是否发生变化，如果变化则更新令牌桶
		bucket.mutex.Lock()
		if bucket.refillRate != qps {
			bucket.refillRate = qps
			bucket.capacity = qps
			if bucket.tokens > bucket.capacity {
				bucket.tokens = bucket.capacity
			}
		}
		bucket.mutex.Unlock()
		return bucket
	}

	// 创建新的令牌桶
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// 双重检查，防止并发创建
	if bucket, exists := rl.buckets[clientID]; exists {
		return bucket
	}

	bucket = NewTokenBucket(qps, qps)
	rl.buckets[clientID] = bucket

	logger.Infof("Created new token bucket for client %s with QPS %d", clientID, qps)
	return bucket
}

// cleanup 清理不活跃的令牌桶
func (rl *RateLimitMiddleware) cleanup() {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟清理一次
	defer ticker.Stop()

	for range ticker.C {
		rl.mutex.Lock()
		now := time.Now()

		for clientID, bucket := range rl.buckets {
			bucket.mutex.Lock()
			// 如果令牌桶超过10分钟没有活动，则删除
			if now.Sub(bucket.lastRefill) > 10*time.Minute {
				delete(rl.buckets, clientID)
				logger.Debugf("Cleaned up inactive token bucket for client %s", clientID)
			}
			bucket.mutex.Unlock()
		}

		rl.mutex.Unlock()
	}
}

// GetBucketStats 获取令牌桶统计信息（用于监控）
func (rl *RateLimitMiddleware) GetBucketStats() map[string]map[string]interface{} {
	rl.mutex.RLock()
	defer rl.mutex.RUnlock()

	stats := make(map[string]map[string]interface{})

	for clientID, bucket := range rl.buckets {
		bucket.mutex.Lock()
		stats[clientID] = map[string]interface{}{
			"capacity":    bucket.capacity,
			"tokens":      bucket.tokens,
			"refill_rate": bucket.refillRate,
			"last_refill": bucket.lastRefill,
		}
		bucket.mutex.Unlock()
	}

	return stats
}
