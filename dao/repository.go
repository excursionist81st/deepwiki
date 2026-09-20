package dao

import (
	"time"

	"deepseek_wiki/model"
)

func CreateRepository(name, url, localPath string) (*model.Repository, error) {
	repo := &model.Repository{
		Name:      name,
		URL:       url,
		LocalPath: localPath,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	err := db.Create(repo).Error
	return repo, err
}

func GetRepositoryByName(name string) (*model.Repository, error) {
	var repo model.Repository
	err := db.Where("name = ?", name).First(&repo).Error
	if err != nil {
		return nil, err
	}
	return &repo, nil
}

func GetRepositoryByID(id uint) (*model.Repository, error) {
	var repo model.Repository
	err := db.Where("id = ?", id).First(&repo).Error
	if err != nil {
		return nil, err
	}
	return &repo, nil
}

func UpdateRepositoryStatus(id uint, status string) error {
	return db.Model(&model.Repository{}).Where("id = ?", id).Update("status", status).Error
}

func UpdateRepositoryHasEmbedding(id uint, hasEmbedding bool) error {
	return db.Model(&model.Repository{}).Where("id = ?", id).Update("has_embedding", hasEmbedding).Error
}

func DeleteRepositoryByName(name string) error {
	var repo model.Repository
	if err := db.Where("name = ?", name).First(&repo).Error; err != nil {
		return nil
	}

	if err := db.Where("repo_id = ?", repo.ID).Delete(&model.CodeChunk{}).Error; err != nil {
		return err
	}

	if err := db.Where("repo_id = ?", repo.ID).Delete(&model.IngestTask{}).Error; err != nil {
		return err
	}

	return db.Delete(&repo).Error
}
