package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ProcessGenericBadRequest(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
}

func ProcessGenericInternalError(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}
