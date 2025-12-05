package worker

import (
	"api-gateway/model"
	"api-gateway/pkg/logger"
	"api-gateway/pkg/queue"
	"api-gateway/repository"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type WorkerPool struct {
	workerCount int
	queue       queue.TaskQueue
	taskRepo    repository.TaskRepository
	httpClient  *http.Client
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewWorkerPool(workerCount int, queue queue.TaskQueue, taskRepo repository.TaskRepository) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		workerCount: workerCount,
		queue:       queue,
		taskRepo:    taskRepo,
		httpClient: &http.Client{
			Timeout: 0, // 不设置全局超时，使用任务的超时配置
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (wp *WorkerPool) Start() {
	logger.Infof("Starting worker pool with %d workers", wp.workerCount)

	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

func (wp *WorkerPool) Stop() {
	logger.Info("Stopping worker pool...")
	wp.cancel()
	wp.wg.Wait()
	logger.Info("Worker pool stopped")
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	logger.Infof("Worker %d started", id)

	for {
		select {
		case <-wp.ctx.Done():
			logger.Infof("Worker %d stopped", id)
			return
		default:
			task, err := wp.queue.Dequeue(wp.ctx)
			if err != nil {
				if err != context.Canceled {
					logger.Errorf("Worker %d failed to dequeue task: %v", id, err)
				}
				return
			}
			if task == nil {
				continue
			}

			// 处理任务
			wp.processTask(id, task)
		}
	}
}

func (wp *WorkerPool) processTask(workerID int, task *model.Task) {
	// 标记任务为处理中， 出队和标记原子操作TODO
	task.MarkProcessing()
	if err := wp.taskRepo.Update(context.Background(), task); err != nil {
		logger.Errorf("Failed to mark task %s as processing: %v", task.TaskID, err)
	}

	// 调用上游服务
	result, statusCode, err := wp.callUpstream(task)

	// 更新任务结果
	if err != nil {
		task.MarkFailed(err.Error()+"|"+result, statusCode)
		logger.Errorf("Worker %d task %s failed: %v", workerID, task.TaskID, err)
	} else {
		task.MarkSuccess(result, statusCode)
		logger.Infof("Worker %d task %s succeeded", workerID, task.TaskID)
	}

	// 保存任务结果
	if err := wp.taskRepo.Update(context.Background(), task); err != nil {
		logger.Errorf("Failed to update task %s: %v", task.TaskID, err)
	}

	// 执行回调
	if task.CallbackURL != "" {
		wp.executeCallback(task)
	}
}

func (wp *WorkerPool) callUpstream(task *model.Task) (string, int, error) {
	req, err := http.NewRequest(task.Method, task.TargetURL, bytes.NewBufferString(task.Body))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range task.Headers {
		req.Header.Set(key, value)
	}

	resp, err := wp.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to call upstream: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return string(body), resp.StatusCode, fmt.Errorf("upstream returned error: %d", resp.StatusCode)
	}

	return string(body), resp.StatusCode, nil
}

func (wp *WorkerPool) executeCallback(task *model.Task) {
	logger.Infof("Executing callback for task %s to %s", task.TaskID, task.CallbackURL)

	callbackData := map[string]interface{}{
		"task_id":      task.TaskID,
		"status":       task.Status,
		"result":       task.Result,
		"error":        task.ErrorMessage,
		"status_code":  task.StatusCode,
		"completed_at": task.CompletedAt,
	}

	payload, err := json.Marshal(callbackData)
	if err != nil {
		logger.Errorf("Failed to marshal callback data for task %s: %v", task.TaskID, err)
		return
	}

	method := task.CallbackMethod
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, task.CallbackURL, bytes.NewBuffer(payload))
	if err != nil {
		logger.Errorf("Failed to create callback request for task %s: %v", task.TaskID, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Task-ID", task.TaskID)

	for key, value := range task.CallbackHeaders {
		req.Header.Set(key, value)
	}

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		resp, err := wp.httpClient.Do(req)

		wp.taskRepo.IncrementCallbackAttempts(context.Background(), task.TaskID)

		if err != nil {
			logger.Errorf("Callback attempt %d for task %s failed: %v", i+1, task.TaskID, err)
			if i < maxRetries-1 {
				time.Sleep(time.Duration(i+1) * 5 * time.Second) // 递增延迟重试
				continue
			}
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logger.Infof("Callback for task %s succeeded", task.TaskID)
			return
		}

		logger.Errorf("Callback attempt %d for task %s returned status %d", i+1, task.TaskID, resp.StatusCode)
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * 5 * time.Second)
		}
	}
}
