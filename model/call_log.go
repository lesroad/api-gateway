package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CallLog represents an API call log entry
type CallLog struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	ClientID  primitive.ObjectID `json:"client_id" bson:"client_id"`
	APIKey    string             `json:"api_key" bson:"api_key"`
	Version   string             `json:"version" bson:"version"`
	Path      string             `json:"path" bson:"path"`
	Method    string             `json:"method" bson:"method"`
	Status    int                `json:"status" bson:"status"`     // HTTP状态码
	Duration  int64              `json:"duration" bson:"duration"` // 响应时间(ms)
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
}

// NewCallLog creates a new call log entry
func NewCallLog(clientID primitive.ObjectID, apiKey, version, path, method string, status int, duration int64) *CallLog {
	return &CallLog{
		ClientID:  clientID,
		APIKey:    apiKey,
		Version:   version,
		Path:      path,
		Method:    method,
		Status:    status,
		Duration:  duration,
		CreatedAt: time.Now(),
	}
}
