package service

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"deepseek_wiki/dao"
	"deepseek_wiki/model"
	"deepseek_wiki/pkg"
)

type TaskManager struct {
	tasks sync.Map
}

var taskManager = &TaskManager{}

func IngestRepo(repoURL, repoName string, filter *pkg.FileFilter) (string, error) {
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

	cloner := &pkg.GitCloner{RepoDir: "./data/repos"}

	dao.UpdateTaskProgress(taskID, 10, "processing")

	repoPath, err := cloner.Clone(repoURL, repoName, filter)
	if err != nil {
		dao.UpdateTaskError(taskID, err.Error())
		dao.UpdateRepositoryStatus(repoID, "failed")
		return
	}

	dao.UpdateTaskProgress(taskID, 50, "processing")

	dao.UpdateRepositoryStatus(repoID, "processing")

	chunks, err := chunkRepository(repoPath, repoID)
	if err != nil {
		dao.UpdateTaskError(taskID, err.Error())
		return
	}

	dao.UpdateTaskProgress(taskID, 80, "processing")

	if err := saveCodeChunks(repoID, chunks); err != nil {
		dao.UpdateTaskError(taskID, err.Error())
		return
	}

	dao.UpdateTaskProgress(taskID, 100, "completed")
	dao.UpdateRepositoryStatus(repoID, "completed")
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
