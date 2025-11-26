package handler

import (
	"api-gateway/model"
	"api-gateway/repository"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// TaskHandler 任务处理器
type TaskHandler struct {
	taskRepo repository.TaskRepository
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(taskRepo repository.TaskRepository) *TaskHandler {
	return &TaskHandler{
		taskRepo: taskRepo,
	}
}

// GetTask 获取任务详情
// GET /api/tasks/:task_id
func (h *TaskHandler) GetTask(c *gin.Context) {
	taskID := c.Param("task_id")

	task, err := h.taskRepo.GetByTaskID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    40400,
			"message": "任务不存在",
		})
		return
	}

	// 检查是否是任务的所有者
	clientInterface, _ := c.Get("client")
	if client, ok := clientInterface.(*model.Client); ok {
		if task.ClientID != client.ID.Hex() {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    40300,
				"message": "无权访问此任务",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": "success",
		"data":    task,
	})
}

// ListTasks 获取任务列表
// GET /api/tasks?limit=10&offset=0
func (h *TaskHandler) ListTasks(c *gin.Context) {
	// 获取分页参数
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	// 获取客户端信息
	clientInterface, exists := c.Get("client")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    40100,
			"message": "未授权",
		})
		return
	}

	client := clientInterface.(*model.Client)

	// 查询任务列表
	tasks, err := h.taskRepo.GetTasksByClient(c.Request.Context(), client.ID.Hex(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    50000,
			"message": "查询任务失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": "success",
		"data": gin.H{
			"tasks":  tasks,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// GetTaskStatus 获取任务状态（简化版）
// GET /api/tasks/:task_id/status
func (h *TaskHandler) GetTaskStatus(c *gin.Context) {
	taskID := c.Param("task_id")

	task, err := h.taskRepo.GetByTaskID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    40400,
			"message": "任务不存在",
		})
		return
	}

	// 检查是否是任务的所有者
	clientInterface, _ := c.Get("client")
	if client, ok := clientInterface.(*model.Client); ok {
		if task.ClientID != client.ID.Hex() {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    40300,
				"message": "无权访问此任务",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    20000,
		"message": "success",
		"data": gin.H{
			"task_id":      task.TaskID,
			"status":       task.Status,
			"created_at":   task.CreatedAt,
			"completed_at": task.CompletedAt,
			"result":       task.Result,
			"error":        task.ErrorMessage,
		},
	})
}
