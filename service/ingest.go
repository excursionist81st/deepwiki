package service

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"deepseek_wiki/dao"
	"deepseek_wiki/model"
	"deepseek_wiki/pkg"
)

type ProgressCallback func(taskID, status string, progress int, errMsg string)

var progressCallback ProgressCallback

func SetProgressCallback(cb ProgressCallback) {
	progressCallback = cb
}

type TaskManager struct {
	tasks sync.Map
}

var taskManager = &TaskManager{}

func IngestRepo(repoURL, repoName string, filter *pkg.FileFilter) (string, error) {
	existingRepo, err := dao.GetRepositoryByName(repoName)
	if err == nil && existingRepo != nil {
		return "", fmt.Errorf("仓库 '%s' 已存在，请使用不同的名称或删除现有仓库", repoName)
	}

	taskID := generateTaskID()

	repo, err := dao.CreateRepository(repoName, repoURL, fmt.Sprintf("./data/repos/%s", repoName))
	if err != nil {
		return "", err
	}

	if err := dao.CreateTask(taskID, repo.ID); err != nil {
		return "", err
	}

	go executeIngest(taskID, repo.ID, repoURL, repoName, filter)

	return taskID, nil
}

func executeIngest(taskID string, repoID uint, repoURL, repoName string, filter *pkg.FileFilter) {
	updateProgress := func(progress int, status string) {
		dao.UpdateTaskProgress(taskID, progress, status)
		if progressCallback != nil {
			progressCallback(taskID, status, progress, "")
		}
	}

	updateProgress(0, "processing")

	cloner := &pkg.GitCloner{RepoDir: "./data/repos"}

	updateProgress(10, "processing")

	repoPath, err := cloner.Clone(repoURL, repoName, filter)
	if err != nil {
		errMsg := sanitizeIngestError(err)
		dao.UpdateTaskError(taskID, errMsg)
		dao.UpdateRepositoryStatus(repoID, "failed")
		if progressCallback != nil {
			progressCallback(taskID, "failed", 0, errMsg)
		}
		return
	}

	updateProgress(50, "processing")

	dao.UpdateRepositoryStatus(repoID, "processing")

	chunks, err := chunkRepository(repoPath, repoID)
	if err != nil {
		errMsg := sanitizeIngestError(err)
		dao.UpdateTaskError(taskID, errMsg)
		if progressCallback != nil {
			progressCallback(taskID, "failed", 50, errMsg)
		}
		return
	}

	updateProgress(80, "processing")

	if err := saveCodeChunks(repoID, chunks); err != nil {
		errMsg := sanitizeIngestError(err)
		dao.UpdateTaskError(taskID, errMsg)
		if progressCallback != nil {
			progressCallback(taskID, "failed", 80, errMsg)
		}
		return
	}

	updateProgress(100, "completed")
	dao.UpdateRepositoryStatus(repoID, "completed")
}

func sanitizeIngestError(err error) string {
	errMsg := err.Error()

	if strings.Contains(errMsg, "TLS handshake timeout") || strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "wsarecv") {
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

func chunkRepository(repoPath string, repoID uint) ([]model.CodeChunk, error) {
	var chunks []model.CodeChunk
	chunker := pkg.NewChunker()

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		fileChunks, err := chunker.ChunkFile(path, repoPath)
		if err != nil {
			return nil
		}
		for i := range fileChunks {
			fileChunks[i].RepoID = repoID
			chunks = append(chunks, fileChunks[i])
		}

		return nil
	})

	return chunks, err
}

func saveCodeChunks(repoID uint, chunks []model.CodeChunk) error {
	for i := range chunks {
		if err := dao.CreateCodeChunk(&chunks[i]); err != nil {
			return err
		}
	}
	return nil
}

func GetTaskStatus(taskID string) (map[string]interface{}, error) {
	task, err := dao.GetTask(taskID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"task_id":    task.ID,
		"status":     task.Status,
		"progress":   task.Progress,
		"error":      task.ErrorMessage,
		"created_at": task.CreatedAt,
		"updated_at": task.UpdatedAt,
	}, nil
}

func generateTaskID() string {
	return fmt.Sprintf("task_%d_%03d", time.Now().UnixNano(), rand.Intn(100))
}
