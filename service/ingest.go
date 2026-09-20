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
	if err := dao.DeleteRepositoryByName(repoName); err != nil {
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
	dao.UpdateTaskProgress(taskID, 0, "processing")
	if progressCallback != nil {
		progressCallback(taskID, "processing", 0, "")
	}

	cloner := &pkg.GitCloner{RepoDir: "./data/repos"}

	dao.UpdateTaskProgress(taskID, 10, "processing")
	if progressCallback != nil {
		progressCallback(taskID, "processing", 10, "")
	}

	repoPath, err := cloner.Clone(repoURL, repoName, filter)
	if err != nil {
		errMsg := sanitizeIngestError(err)
		dao.UpdateTaskError(taskID, errMsg)
		dao.DeleteRepositoryByName(repoName)
		if progressCallback != nil {
			progressCallback(taskID, "failed", 0, errMsg)
		}
		return
	}

	dao.UpdateTaskProgress(taskID, 50, "processing")
	if progressCallback != nil {
		progressCallback(taskID, "processing", 50, "")
	}

	dao.UpdateRepositoryStatus(repoID, "processing")

	chunks, err := chunkRepository(repoPath, repoID)
	if err != nil {
		errMsg := sanitizeIngestError(err)
		dao.UpdateTaskError(taskID, errMsg)
		dao.DeleteRepositoryByName(repoName)
		if progressCallback != nil {
			progressCallback(taskID, "failed", 50, errMsg)
		}
		return
	}

	dao.UpdateTaskProgress(taskID, 80, "processing")
	if progressCallback != nil {
		progressCallback(taskID, "processing", 80, "")
	}

	if err := saveCodeChunks(repoID, chunks); err != nil {
		errMsg := sanitizeIngestError(err)
		dao.UpdateTaskError(taskID, errMsg)
		dao.DeleteRepositoryByName(repoName)
		if progressCallback != nil {
			progressCallback(taskID, "failed", 80, errMsg)
		}
		return
	}

	dao.UpdateTaskProgress(taskID, 100, "completed")
	if progressCallback != nil {
		progressCallback(taskID, "completed", 100, "")
	}
	dao.UpdateRepositoryStatus(repoID, "completed")
}

func sanitizeIngestError(err error) string {
	errMsg := err.Error()

	if strings.Contains(errMsg, "TLS handshake timeout") || strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "wsarecv") {
		return fmt.Sprintf("网络连接超时: %s", errMsg)
	}
	if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "no such host") {
		return fmt.Sprintf("无法连接到服务器: %s", errMsg)
	}
	if strings.Contains(errMsg, "repository not found") || strings.Contains(errMsg, "404") {
		return fmt.Sprintf("仓库不存在: %s", errMsg)
	}
	if strings.Contains(errMsg, "authentication required") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") {
		return fmt.Sprintf("认证失败: %s", errMsg)
	}
	if strings.Contains(errMsg, "already exists") {
		return fmt.Sprintf("目录已存在: %s", errMsg)
	}

	return fmt.Sprintf("操作失败: %s", errMsg)
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
