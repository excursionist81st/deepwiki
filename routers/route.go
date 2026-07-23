package routers

import (
	"deepseek_wiki/handlers"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	r.POST("/api/ingest", handlers.IngestHandler)
	r.GET("/api/ingest/:id/status", handlers.StatusHandler)
	r.POST("/api/ask", handlers.AskStreamHandler)

}
