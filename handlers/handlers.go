package handlers

import (
	"net/http"
	"strings"

	"deepseek_wiki/config"
	"deepseek_wiki/model"
	gitclone "deepseek_wiki/pkg"
	"deepseek_wiki/service"

	"github.com/gin-gonic/gin"
)

func sanitizeError(err error) string {
	errMsg := err.Error()

	if strings.Contains(errMsg, "TLS handshake timeout") || strings.Contains(errMsg, "timeout") {
		return "网络连接超时，请检查网络或使用镜像地址"
	}
	if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host") {
		return "无法连接到服务器，请检查网络连接"
	}
	if strings.Contains(errMsg, "repository not found") || strings.Contains(errMsg, "404") {
		return "仓库不存在或无访问权限"
	}
	if strings.Contains(errMsg, "authentication required") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") {
		return "认证失败，请检查访问权限"
	}

	return "操作失败，请稍后重试"
}

func IngestHandler(c *gin.Context) {
	var req model.IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	filter := gitclone.NewFileFilter()
	if len(req.IncludeExts) > 0 {
		filter.IncludeExts = req.IncludeExts
	}
	if len(req.IncludeFiles) > 0 {
		filter.IncludeFiles = req.IncludeFiles
	}
	if len(req.ExcludeDirs) > 0 {
		filter.ExcludeDirs = append(filter.ExcludeDirs, req.ExcludeDirs...)
	}
	if len(req.ExcludeExts) > 0 {
		filter.ExcludeExts = append(filter.ExcludeExts, req.ExcludeExts...)
	}

	taskID, err := service.IngestRepo(req.RepoURL, req.RepoName, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": sanitizeError(err)})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数格式错误"})
		return
	}

	apiKey := req.ApiKey
	baseURL := req.BaseURL
	alibabaKey := req.AlibabaAPIKey
	alibabaURL := req.AlibabaBaseURL

	if apiKey == "" {
		apiKey = config.GetString("deepseek.api_key")
		baseURL = config.GetString("deepseek.base_url")
	} else if baseURL == "" {
		baseURL = config.GetString("deepseek.base_url")
	}

	if alibabaKey == "" {
		alibabaKey = config.GetString("alibaba.api_key")
		alibabaURL = config.GetString("alibaba.base_url")
	} else if alibabaURL == "" {
		alibabaURL = config.GetString("alibaba.base_url")
	}

	qaService := service.NewQAServiceWithAlibaba(apiKey, baseURL, alibabaKey, alibabaURL)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	err := qaService.AskStream(req.RepoName, req.Question, func(chunk string) {
		c.Writer.Write([]byte("data: " + chunk + "\n\n"))
		c.Writer.Flush()
	})

	if err != nil {
		c.Writer.Write([]byte("data: 错误: " + sanitizeError(err) + "\n\n"))
		c.Writer.Flush()
	}
}
