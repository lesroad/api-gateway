package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"    // 等待处理
	TaskStatusProcessing TaskStatus = "processing" // 处理中
	TaskStatusSuccess    TaskStatus = "success"    // 成功
	TaskStatusFailed     TaskStatus = "failed"     // 失败
	TaskStatusTimeout    TaskStatus = "timeout"    // 超时
)

// Task 异步任务
type Task struct {
	ID       primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TaskID   string             `json:"task_id" bson:"task_id"`     // 唯一任务ID（用于查询）
	ClientID string             `json:"client_id" bson:"client_id"` // 客户端ID
	APIKey   string             `json:"api_key" bson:"api_key"`     // API Key

	// 请求信息
	Method    string            `json:"method" bson:"method"`         // HTTP 方法
	Path      string            `json:"path" bson:"path"`             // 请求路径
	Headers   map[string]string `json:"headers" bson:"headers"`       // 请求头
	Body      string            `json:"body" bson:"body"`             // 请求体
	TargetURL string            `json:"target_url" bson:"target_url"` // 目标URL

	// 回调信息
	CallbackURL     string            `json:"callback_url" bson:"callback_url"`         // 回调URL
	CallbackMethod  string            `json:"callback_method" bson:"callback_method"`   // 回调方法（默认POST）
	CallbackHeaders map[string]string `json:"callback_headers" bson:"callback_headers"` // 回调时额外的headers

	// 任务状态
	Status       TaskStatus `json:"status" bson:"status"`                                   // 任务状态
	Result       string     `json:"result,omitempty" bson:"result,omitempty"`               // 处理结果
	ErrorMessage string     `json:"error_message,omitempty" bson:"error_message,omitempty"` // 错误信息
	StatusCode   int        `json:"status_code,omitempty" bson:"status_code,omitempty"`     // HTTP 状态码

	// 回调状态
	CallbackAttempts int       `json:"callback_attempts" bson:"callback_attempts"`                   // 回调尝试次数
	CallbackStatus   string    `json:"callback_status,omitempty" bson:"callback_status,omitempty"`   // 回调状态
	LastCallbackAt   time.Time `json:"last_callback_at,omitempty" bson:"last_callback_at,omitempty"` // 最后回调时间

	// 时间信息
	CreatedAt   time.Time  `json:"created_at" bson:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty" bson:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	ExpireAt    time.Time  `json:"expire_at" bson:"expire_at"` // 过期时间（用于自动清理）
}

// NewTask 创建新任务
func NewTask(taskID, clientID, apiKey, method, path, targetURL, callbackURL string,
	headers map[string]string, body string) *Task {
	now := time.Now()
	return &Task{
		TaskID:         taskID,
		ClientID:       clientID,
		APIKey:         apiKey,
		Method:         method,
		Path:           path,
		Headers:        headers,
		Body:           body,
		TargetURL:      targetURL,
		CallbackURL:    callbackURL,
		CallbackMethod: "POST", // 默认POST
		Status:         TaskStatusPending,
		CreatedAt:      now,
		ExpireAt:       now.Add(24 * time.Hour), // 24小时后过期
	}
}

// IsCompleted 任务是否已完成（成功或失败）
func (t *Task) IsCompleted() bool {
	return t.Status == TaskStatusSuccess ||
		t.Status == TaskStatusFailed ||
		t.Status == TaskStatusTimeout
}

// CanRetry 是否可以重试回调
func (t *Task) CanRetry() bool {
	return t.CallbackAttempts < 3 && t.IsCompleted()
}

func (t *Task) MarkProcessing() {
	now := time.Now()
	t.Status = TaskStatusProcessing
	t.StartedAt = &now
}

// MarkSuccess 标记为成功
func (t *Task) MarkSuccess(result string, statusCode int) {
	now := time.Now()
	t.Status = TaskStatusSuccess
	t.Result = result
	t.StatusCode = statusCode
	t.CompletedAt = &now
}

// MarkFailed 标记为失败
func (t *Task) MarkFailed(errorMsg string, statusCode int) {
	now := time.Now()
	t.Status = TaskStatusFailed
	t.ErrorMessage = errorMsg
	t.StatusCode = statusCode
	t.CompletedAt = &now
}

// MarkTimeout 标记为超时
func (t *Task) MarkTimeout() {
	now := time.Now()
	t.Status = TaskStatusTimeout
	t.ErrorMessage = "Task execution timeout"
	t.CompletedAt = &now
}
