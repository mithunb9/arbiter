package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine) {
	r.GET("/health", handleShallow)
}

func handleShallow(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
