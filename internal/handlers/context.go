package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func getUserID(c *gin.Context) (string, bool) {
	v, exists := c.Get("user_id")
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return "", false
	}
	id, ok := v.(string)
	if !ok || id == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return "", false
	}
	return id, true
}
