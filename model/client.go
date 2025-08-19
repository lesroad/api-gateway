package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Client represents an API client with billing information
type Client struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name       string             `json:"name" bson:"name"`
	APIKey     string             `json:"api_key" bson:"api_key"`
	Secret     string             `json:"-" bson:"secret"`                // 签名密钥，不返回给客户端
	Version    string             `json:"version" bson:"version"`         // 绑定的API版本
	CallCount  int                `json:"call_count" bson:"call_count"`   // 剩余调用次数
	TotalCount int                `json:"total_count" bson:"total_count"` // 总购买次数
	QPS        int                `json:"qps" bson:"qps"`                 // 每秒请求数限制
	Status     int                `json:"status" bson:"status"`           // 0:禁用 1:正常
	CreatedAt  time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at" bson:"updated_at"`
}

// ClientStatus constants
const (
	ClientStatusDisabled = 0 // 禁用
	ClientStatusActive   = 1 // 正常
)

// NewClient creates a new client with default values
func NewClient(name, apiKey, secret, version string, initialCallCount int) *Client {
	now := time.Now()
	return &Client{
		Name:       name,
		APIKey:     apiKey,
		Secret:     secret,
		Version:    version,
		CallCount:  initialCallCount,
		TotalCount: initialCallCount,
		QPS:        10, // 默认QPS限制为10
		Status:     ClientStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// IsActive returns true if the client is active
func (c *Client) IsActive() bool {
	return c.Status == ClientStatusActive
}

// HasCallsRemaining returns true if the client has remaining calls
func (c *Client) HasCallsRemaining() bool {
	return c.CallCount > 0
}

// DecrementCallCount decreases the call count by 1
func (c *Client) DecrementCallCount() {
	if c.CallCount > 0 {
		c.CallCount--
	}
	c.UpdatedAt = time.Now()
}

// AddCallCount increases the call count and total count
func (c *Client) AddCallCount(count int) {
	c.CallCount += count
	c.TotalCount += count
	c.UpdatedAt = time.Now()
}
