package handlers

import (
	"net/http"

	"deepseek_wiki/config"
	"deepseek_wiki/model"
	gitclone "deepseek_wiki/pkg"
	"deepseek_wiki/service"

	"github.com/gin-gonic/gin"
)

func IngestHandler(c *gin.Context) {
	var req model.IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filter := gitclone.NewFileFilter()
	if len(req.IncludeExts) > 0 {
		filter.IncludeExts = req.IncludeExts
	}
	if len(req.ExcludeDirs) > 0 {
		filter.ExcludeDirs = append(filter.ExcludeDirs, req.ExcludeDirs...)
	}
	if len(req.ExcludeExts) > 0 {
		filter.ExcludeExts = append(filter.ExcludeExts, req.ExcludeExts...)
	}

	taskID, err := service.IngestRepo(req.RepoURL, req.RepoName, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.IngestResponse{
		TaskID:  taskID,
		Message: "摄取任务已提交，正在异步处理",
	})
}

func StatusHandler(c *gin.Context) {
	taskID := c.Param("id")

	status, err := service.GetTaskStatus(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, status)
}

func AskStreamHandler(c *gin.Context) {
	var req model.AskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	apiKey := req.ApiKey
	baseURL := req.BaseURL

	if apiKey == "" {
		apiKey = config.GetString("deepseek.api_key")
	}
	if baseURL == "" {
		baseURL = config.GetString("deepseek.base_url")
	}

	qaService := service.NewQAService(apiKey, baseURL)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	err := qaService.AskStream(req.RepoName, req.Question, func(chunk string) {
		c.Writer.Write([]byte(chunk))
		c.Writer.Flush()
	})

	if err != nil {
		c.Writer.Write([]byte("\n\n错误: " + err.Error()))
		c.Writer.Flush()
	}
}
