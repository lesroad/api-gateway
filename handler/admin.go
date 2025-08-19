package handler

import (
	"api-gateway/model"
	"api-gateway/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdminHandler handles admin operations
type AdminHandler struct {
	clientService service.ClientServiceInterface
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(clientService service.ClientServiceInterface) *AdminHandler {
	return &AdminHandler{
		clientService: clientService,
	}
}

// CreateClientRequest represents the request to create a new client
type CreateClientRequest struct {
	Name             string `json:"name" binding:"required"`
	Version          string `json:"version" binding:"required"`
	InitialCallCount int    `json:"initial_call_count" binding:"min=0"`
	QPS              int    `json:"qps" binding:"min=1"`
}

// CreateClientResponse represents the response after creating a client
type CreateClientResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	APIKey           string `json:"api_key"`
	Secret           string `json:"secret"` // 仅创建时返回一次
	Version          string `json:"version"`
	InitialCallCount int    `json:"initial_call_count"`
	QPS              int    `json:"qps"`
	Status           int    `json:"status"`
	CreatedAt        string `json:"created_at"`
}

// RechargeRequest represents the request to recharge a client
type RechargeRequest struct {
	CallCount int `json:"call_count" binding:"required,min=1"`
}

// UpdateStatusRequest represents the request to update client status
type UpdateStatusRequest struct {
	Status int `json:"status" binding:"min=0,max=1"`
}

// UpdateQPSRequest represents the request to update client QPS
type UpdateQPSRequest struct {
	QPS int `json:"qps" binding:"required,min=1,max=1000"`
}

// StatsResponse represents basic statistics
type StatsResponse struct {
	TotalClients    int64 `json:"total_clients"`
	ActiveClients   int64 `json:"active_clients"`
	DisabledClients int64 `json:"disabled_clients"`
	TotalCallsUsed  int64 `json:"total_calls_used"`
}

// CreateClient creates a new API client
func (h *AdminHandler) CreateClient(c *gin.Context) {
	var req CreateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    40001,
			Message: "Invalid request parameters",
			Error:   err.Error(),
		})
		return
	}

	client, err := h.clientService.CreateClient(c.Request.Context(), req.Name, req.Version, req.InitialCallCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    50001,
			Message: "Failed to create client",
			Error:   err.Error(),
		})
		return
	}

	// 如果请求中指定了QPS，则更新客户的QPS设置
	if req.QPS > 0 {
		err = h.clientService.UpdateClientQPS(c.Request.Context(), client.ID, req.QPS)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Code:    50001,
				Message: "Failed to set client QPS",
				Error:   err.Error(),
			})
			return
		}
		client.QPS = req.QPS
	}

	response := CreateClientResponse{
		ID:               client.ID.Hex(),
		Name:             client.Name,
		APIKey:           client.APIKey,
		Secret:           client.Secret,
		Version:          client.Version,
		InitialCallCount: client.CallCount,
		QPS:              client.QPS,
		Status:           client.Status,
		CreatedAt:        client.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	c.JSON(http.StatusCreated, response)
}

// ListClients lists all clients with pagination
func (h *AdminHandler) ListClients(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	// Validate pagination parameters
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	clients, err := h.clientService.ListClients(c.Request.Context(), offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    50002,
			Message: "Failed to retrieve clients",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"clients": clients,
		"offset":  offset,
		"limit":   limit,
		"count":   len(clients),
	})
}

// GetClient retrieves a specific client by ID
func (h *AdminHandler) GetClient(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    40002,
			Message: "Invalid client ID format",
			Error:   err.Error(),
		})
		return
	}

	client, err := h.clientService.GetClientByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Code:    40401,
			Message: "Client not found",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, client)
}

// RechargeClient adds call count to a client
func (h *AdminHandler) RechargeClient(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    40002,
			Message: "Invalid client ID format",
			Error:   err.Error(),
		})
		return
	}

	var req RechargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    40003,
			Message: "Invalid recharge parameters",
			Error:   err.Error(),
		})
		return
	}

	err = h.clientService.RechargeClient(c.Request.Context(), id, req.CallCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    50003,
			Message: "Failed to recharge client",
			Error:   err.Error(),
		})
		return
	}

	// Return updated client information
	client, err := h.clientService.GetClientByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    50004,
			Message: "Failed to retrieve updated client",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Client recharged successfully",
		"client":  client,
	})
}

// UpdateClientStatus updates the status of a client (enable/disable)
func (h *AdminHandler) UpdateClientStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    40002,
			Message: "Invalid client ID format",
			Error:   err.Error(),
		})
		return
	}

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    40004,
			Message: "Invalid status parameters",
			Error:   err.Error(),
		})
		return
	}

	err = h.clientService.UpdateClientStatus(c.Request.Context(), id, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    50005,
			Message: "Failed to update client status",
			Error:   err.Error(),
		})
		return
	}

	// Return updated client information
	client, err := h.clientService.GetClientByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    50006,
			Message: "Failed to retrieve updated client",
			Error:   err.Error(),
		})
		return
	}

	statusText := "active"
	if req.Status == model.ClientStatusDisabled {
		statusText = "disabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Client status updated to " + statusText,
		"client":  client,
	})
}

// GetClientCallLogs retrieves call logs for a specific client
func (h *AdminHandler) GetClientCallLogs(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    40002,
			Message: "Invalid client ID format",
			Error:   err.Error(),
		})
		return
	}

	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	// Validate pagination parameters
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	logs, err := h.clientService.GetClientCallLogs(c.Request.Context(), id, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    50007,
			Message: "Failed to retrieve call logs",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":   logs,
		"offset": offset,
		"limit":  limit,
		"count":  len(logs),
	})
}

// GetStats retrieves basic statistics
func (h *AdminHandler) GetStats(c *gin.Context) {
	clients, err := h.clientService.ListClients(c.Request.Context(), 0, 1000) // Get up to 1000 clients
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    50008,
			Message: "Failed to retrieve statistics",
			Error:   err.Error(),
		})
		return
	}

	stats := StatsResponse{
		TotalClients:    int64(len(clients)),
		ActiveClients:   0,
		DisabledClients: 0,
		TotalCallsUsed:  0,
	}

	for _, client := range clients {
		if client.Status == 1 { // Active
			stats.ActiveClients++
		} else {
			stats.DisabledClients++
		}
		stats.TotalCallsUsed += int64(client.TotalCount - client.CallCount)
	}

	c.JSON(http.StatusOK, stats)
}

// UpdateClientQPS updates a client's QPS limit
func (h *AdminHandler) UpdateClientQPS(c *gin.Context) {
	clientID := c.Param("id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    40001,
			Message: "Client ID is required",
		})
		return
	}

	objectID, err := primitive.ObjectIDFromHex(clientID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    40001,
			Message: "Invalid client ID format",
			Error:   err.Error(),
		})
		return
	}

	var req UpdateQPSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    40001,
			Message: "Invalid request parameters",
			Error:   err.Error(),
		})
		return
	}

	err = h.clientService.UpdateClientQPS(c.Request.Context(), objectID, req.QPS)
	if err != nil {
		if err.Error() == "client not found" {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Code:    40404,
				Message: "Client not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    50007,
			Message: "Failed to update client QPS",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Client QPS updated successfully",
		"client_id": clientID,
		"qps":       req.QPS,
	})
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
