package dao

import (
	"time"

	"deepseek_wiki/model"
)

func CreateTask(taskID string, repoID uint) error {
	task := &model.IngestTask{
		ID:        taskID,
		RepoID:    repoID,
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return db.Create(task).Error
}

func GetTask(taskID string) (*model.IngestTask, error) {
	var task model.IngestTask
	err := db.Where("id = ?", taskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func UpdateTaskProgress(taskID string, progress int, status string) error {
	return db.Model(&model.IngestTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"progress":   progress,
		"status":     status,
		"updated_at": time.Now(),
	}).Error
}

func UpdateTaskError(taskID string, errMsg string) error {
	return db.Model(&model.IngestTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":        "failed",
		"error_message": errMsg,
		"updated_at":    time.Now(),
	}).Error
}
