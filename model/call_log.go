package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CallLog represents an API call log entry
type CallLog struct {
	ID           primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	ClientID     primitive.ObjectID `json:"client_id" bson:"client_id"`
	APIKey       string             `json:"api_key" bson:"api_key"`
	Path         string             `json:"path" bson:"path"`
	Status       int                `json:"status" bson:"status"`                         // HTTP状态码
	Duration     int64              `json:"duration" bson:"duration"`                     // 响应时间(ms)
	RequestBody  string             `json:"request_body" bson:"request_body,omitempty"`   // 请求参数
	ResponseBody string             `json:"response_body" bson:"response_body,omitempty"` // 响应参数
	CreatedAt    time.Time          `json:"created_at" bson:"created_at"`
}

// NewCallLog creates a new call log entry
func NewCallLog(clientID primitive.ObjectID, apiKey, path string, status int, duration int64) *CallLog {
	return &CallLog{
		ClientID:  clientID,
		APIKey:    apiKey,
		Path:      path,
		Status:    status,
		Duration:  duration,
		CreatedAt: time.Now(),
	}
}

// NewCallLogWithParams creates a new call log entry with request and response parameters
func NewCallLogWithParams(clientID primitive.ObjectID, apiKey, path string, status int, duration int64, requestBody, responseBody string) *CallLog {
	return &CallLog{
		ClientID:     clientID,
		APIKey:       apiKey,
		Path:         path,
		Status:       status,
		Duration:     duration,
		RequestBody:  requestBody,
		ResponseBody: responseBody,
		CreatedAt:    time.Now(),
	}
}
