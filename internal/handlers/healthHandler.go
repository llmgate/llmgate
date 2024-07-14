package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) IsHealthy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"health": "true",
	})
}
