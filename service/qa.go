package service

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"deepseek_wiki/config"
	"deepseek_wiki/dao"
	"deepseek_wiki/deepseek"
	"deepseek_wiki/model"
)

type QAService struct {
	client *deepseek.Client
}

func NewQAService(apiKey, baseURL string) *QAService {
	return &QAService{
		client: deepseek.NewClient(apiKey, baseURL),
	}
}

func (s *QAService) AskStream(repoName, question string, onChunk func(chunk string)) error {
	repo, err := dao.GetRepositoryByName(repoName)
	if err != nil {
		return err
	}

	chunks, err := dao.GetCodeChunksByRepo(repo.ID)
	if err != nil {
		return err
	}

	topK := config.GetInt("qa.top_k")
	if topK <= 0 {
		topK = 10
	}

	relevantChunks := s.retrieveRelevantChunks(repo, question, chunks, topK)

	memory := s.readMemory()
	prompt := s.buildPromptWithMemory(question, relevantChunks, memory)

	systemPrompt := "你是一个代码助手，根据提供的代码片段回答问题。回答时请明确指出引用的文件和行号。\n\n"
	systemPrompt += "如果需要更新对话记忆（例如记录重要的代码结构、API用法、用户偏好等），请在回答末尾使用以下格式：\n"
	systemPrompt += "<memory_update>\n新的记忆内容\n</memory_update>\n"
	systemPrompt += "记忆内容将追加到 data/memory.md 文件中。"

	messages := []deepseek.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	var fullResponse strings.Builder
	wrappedOnChunk := func(chunk string) {
		fullResponse.WriteString(chunk)
		onChunk(chunk)
	}

	err = s.client.ChatStream(messages, wrappedOnChunk)
	if err != nil {
		return err
	}

	response := fullResponse.String()
	_, memoryUpdate, hasUpdate := s.extractMemoryUpdate(response)
	if hasUpdate && memoryUpdate != "" {
		currentMemory := s.readMemory()
		updatedMemory := currentMemory + "\n\n" + memoryUpdate + "\n"
		s.updateMemory(updatedMemory)
	}

	if hasUpdate {
		onChunk("\n\n[记忆已更新]")
	}

	return nil
}

func (s *QAService) retrieveRelevantChunks(repo *model.Repository, question string, chunks []model.CodeChunk, topK int) []model.CodeChunk {
	if len(chunks) == 0 {
		return []model.CodeChunk{}
	}

	if !repo.HasEmbedding {
		successCount, totalCount := s.embedAllChunks(chunks)
		if successCount == totalCount {
			dao.UpdateRepositoryHasEmbedding(repo.ID, true)
		}
	}

	questionEmb, err := s.embeddingWithRetry(question, 3)
	if err != nil {
		return chunks[:min(topK, len(chunks))]
	}

	scores := make([]struct {
		chunk model.CodeChunk
		score float64
	}, 0, len(chunks))

	for i := range chunks {
		chunkEmb, err := dao.GetEmbeddingFromChunk(&chunks[i])
		if err != nil || chunkEmb == nil {
			continue
		}
		score := cosineSimilarity(questionEmb, chunkEmb)
		scores = append(scores, struct {
			chunk model.CodeChunk
			score float64
		}{chunks[i], score})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	result := make([]model.CodeChunk, 0, topK)
	for i := 0; i < topK && i < len(scores); i++ {
		result = append(result, scores[i].chunk)
	}

	return result
}

func (s *QAService) embedAllChunks(chunks []model.CodeChunk) (successCount, totalCount int) {
	totalCount = len(chunks)
	for i := range chunks {
		switch chunks[i].EmbeddingStatus {
		case "success":
			successCount++
		case "pending", "failed":
			enhancedContent := s.buildEnhancedContent(chunks[i])
			embedding, err := s.embeddingWithRetry(enhancedContent, 3)
			if err != nil {
				dao.UpdateCodeChunkEmbeddingFailed(chunks[i].ID, err.Error())
				continue
			}
			if dao.UpdateCodeChunkEmbedding(chunks[i].ID, embedding) == nil {
				successCount++
			}
		}
	}
	return
}

func (s *QAService) buildEnhancedContent(chunk model.CodeChunk) string {
	var builder strings.Builder

	if chunk.SymbolName != "" {
		builder.WriteString("Function: ")
		builder.WriteString(chunk.SymbolName)
		builder.WriteString("\n")
	}

	if chunk.Signature != "" {
		builder.WriteString("Signature: ")
		builder.WriteString(chunk.Signature)
		builder.WriteString("\n")
	}

	if chunk.ParentSymbol != "" {
		builder.WriteString("Class: ")
		builder.WriteString(chunk.ParentSymbol)
		builder.WriteString("\n")
	}

	builder.WriteString("Code:\n")
	builder.WriteString(chunk.Content)

	return builder.String()
}

func (s *QAService) embeddingWithRetry(text string, maxRetries int) ([]float64, error) {
	var embedding []float64
	var err error

	for i := 0; i < maxRetries; i++ {
		embedding, err = s.client.Embedding(text)
		if err == nil {
			return embedding, nil
		}
		if i < maxRetries-1 {
			time.Sleep(time.Second * time.Duration(i+1))
		}
	}

	return nil, err
}

func (s *QAService) buildPromptWithMemory(question string, chunks []model.CodeChunk, memory string) string {
	var sb strings.Builder

	if memory != "" {
		sb.WriteString("=== 对话记忆 ===\n")
		sb.WriteString(memory)
		sb.WriteString("\n=== 记忆结束 ===\n\n")
	}

	sb.WriteString("以下是相关代码片段：\n\n")

	for i, chunk := range chunks {
		sb.WriteString(fmt.Sprintf("【代码片段 %d】\n", i+1))
		sb.WriteString(fmt.Sprintf("文件：%s\n", chunk.FilePath))
		sb.WriteString(fmt.Sprintf("行号：%d-%d\n", chunk.StartLine, chunk.EndLine))
		sb.WriteString(fmt.Sprintf("语言：%s\n", chunk.Language))
		sb.WriteString(fmt.Sprintf("代码：\n```\n%s\n```\n\n", chunk.Content))
	}

	sb.WriteString(fmt.Sprintf("问题：%s\n\n", question))
	sb.WriteString("请根据以上代码和对话记忆回答问题，并在回答中明确指出引用了哪个文件的哪些行。")

	return sb.String()
}

func (s *QAService) extractReferences(chunks []model.CodeChunk) []Reference {
	refs := make([]Reference, 0, len(chunks))
	for _, chunk := range chunks {
		refs = append(refs, Reference{
			FilePath:  chunk.FilePath,
			StartLine: chunk.StartLine,
			EndLine:   chunk.EndLine,
			Language:  chunk.Language,
		})
	}
	return refs
}

type Answer struct {
	Answer     string      `json:"answer"`
	References []Reference `json:"references"`
}

type Reference struct {
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Language  string `json:"language"`
}

func cosineSimilarity(a, b []float64) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getMemoryFilePath() string {
	path := config.GetString("memory.file_path")
	if path == "" {
		path = "data/memory.md"
	}
	return path
}

func (s *QAService) readMemory() string {
	data, err := os.ReadFile(getMemoryFilePath())
	if err != nil {
		return ""
	}
	return string(data)
}

func (s *QAService) updateMemory(newContent string) error {
	return os.WriteFile(getMemoryFilePath(), []byte(newContent), 0644)
}

func (s *QAService) extractMemoryUpdate(response string) (answer string, memoryUpdate string, hasUpdate bool) {
	startTag := "<memory_update>"
	endTag := "</memory_update>"

	startIdx := strings.Index(response, startTag)
	if startIdx == -1 {
		return response, "", false
	}

	endIdx := strings.Index(response, endTag)
	if endIdx == -1 || endIdx <= startIdx {
		return response, "", false
	}

	memoryUpdate = strings.TrimSpace(response[startIdx+len(startTag) : endIdx])
	answer = strings.TrimSpace(response[:startIdx] + response[endIdx+len(endTag):])

	return answer, memoryUpdate, true
}
