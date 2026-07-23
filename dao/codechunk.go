package dao

import (
	"encoding/json"

	"deepseek_wiki/model"
)

func CreateCodeChunk(chunk *model.CodeChunk) error {
	return db.Create(chunk).Error
}

func GetCodeChunksByRepo(repoID uint) ([]model.CodeChunk, error) {
	var chunks []model.CodeChunk
	err := db.Where("repo_id = ?", repoID).Find(&chunks).Error
	return chunks, err
}

func UpdateCodeChunkEmbedding(chunkID uint, embedding []float64) error {
	data, err := json.Marshal(embedding)
	if err != nil {
		return err
	}
	return db.Model(&model.CodeChunk{}).Where("id = ?", chunkID).Updates(map[string]interface{}{
		"embedding":        string(data),
		"embedding_status": "success",
		"embedding_error":  "",
	}).Error
}

func UpdateCodeChunkEmbeddingFailed(chunkID uint, errMsg string) error {
	return db.Model(&model.CodeChunk{}).Where("id = ?", chunkID).Updates(map[string]interface{}{
		"embedding_status": "failed",
		"embedding_error":  errMsg,
	}).Error
}

func GetEmbeddingFromChunk(chunk *model.CodeChunk) ([]float64, error) {
	if chunk.Embedding == "" {
		return nil, nil
	}
	var embedding []float64
	err := json.Unmarshal([]byte(chunk.Embedding), &embedding)
	return embedding, err
}
