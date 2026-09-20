package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"deepseek_wiki/config"
	"deepseek_wiki/dao"
	"deepseek_wiki/deepseek"
	"deepseek_wiki/model"
	"deepseek_wiki/pkg"
)

type QAService struct {
	client *deepseek.Client
}

func NewQAService(apiKey, baseURL string) *QAService {
	alibabaKey := config.GetString("alibaba.api_key")
	alibabaURL := config.GetString("alibaba.base_url")
	return &QAService{
		client: deepseek.NewClientWithAlibaba(apiKey, baseURL, alibabaKey, alibabaURL),
	}
}

func NewQAServiceWithAlibaba(apiKey, baseURL, alibabaKey, alibabaURL string) *QAService {
	return &QAService{
		client: deepseek.NewClientWithAlibaba(apiKey, baseURL, alibabaKey, alibabaURL),
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

	onChunk("\n\n" + s.formatReferences(relevantChunks))

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

		var err error
		chunks, err = dao.GetCodeChunksByRepo(repo.ID)
		if err != nil {
			return []model.CodeChunk{}
		}
	}

	questionEmb, err := s.embeddingWithRetry(question, 3)
	if err != nil {
		return chunks[:min(topK, len(chunks))]
	}

	vectorScores := pkg.VectorSearch(questionEmb, chunks)
	keywordScores := pkg.KeywordSearch(question, chunks)
	hybridScores := pkg.HybridSearch(vectorScores, keywordScores, chunks)

	reranked := pkg.Rerank(question, hybridScores, topK)

	return reranked
}

func (s *QAService) embedAllChunks(chunks []model.CodeChunk) (successCount, totalCount int) {
	totalCount = len(chunks)

	var pendingChunks []model.CodeChunk
	for i := range chunks {
		if chunks[i].EmbeddingStatus == "success" {
			successCount++
		} else if chunks[i].EmbeddingStatus == "pending" || chunks[i].EmbeddingStatus == "failed" {
			pendingChunks = append(pendingChunks, chunks[i])
		}
	}

	if len(pendingChunks) == 0 {
		return
	}

	batchSize := config.GetInt("qa.batch_size")
	if batchSize <= 0 {
		batchSize = 25
	}

	maxConcurrency := config.GetInt("qa.max_concurrency")
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}

	maxTextLen := config.GetInt("qa.max_text_len")
	if maxTextLen <= 0 {
		maxTextLen = 7000
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	batches := make([][]model.CodeChunk, 0)
	for i := 0; i < len(pendingChunks); i += batchSize {
		end := i + batchSize
		if end > len(pendingChunks) {
			end = len(pendingChunks)
		}
		batches = append(batches, pendingChunks[i:end])
	}

	sem := make(chan struct{}, maxConcurrency)

	for batchIdx, batch := range batches {
		wg.Add(1)
		go func(batchIdx int, batchChunks []model.CodeChunk) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			var validChunks []model.CodeChunk
			var texts []string
			for _, chunk := range batchChunks {
				text := s.buildEnhancedContent(chunk)
				if len(text) > maxTextLen {
					dao.UpdateCodeChunkEmbeddingFailed(chunk.ID, "文本过长，跳过向量化")
					continue
				}
				validChunks = append(validChunks, chunk)
				texts = append(texts, text)
			}

			if len(texts) == 0 {
				return
			}

			embeddings, err := s.client.EmbeddingBatch(texts)

			if err != nil || len(embeddings) != len(texts) {
				for _, chunk := range validChunks {
					embedding, singleErr := s.embeddingWithRetry(s.buildEnhancedContent(chunk), 3)
					if singleErr != nil {
						errMsg := "单个向量化失败"
						errMsg = singleErr.Error()
						dao.UpdateCodeChunkEmbeddingFailed(chunk.ID, errMsg)
						continue
					}
					if dao.UpdateCodeChunkEmbedding(chunk.ID, embedding) == nil {
						mu.Lock()
						successCount++
						mu.Unlock()
					}
				}
				return
			}

			for i, embedding := range embeddings {
				if len(embedding) == 0 {
					dao.UpdateCodeChunkEmbeddingFailed(validChunks[i].ID, "向量化结果为空")
					continue
				}
				if dao.UpdateCodeChunkEmbedding(validChunks[i].ID, embedding) == nil {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
		}(batchIdx, batch)
	}

	wg.Wait()

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
		if err == nil && embedding != nil {
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

func (s *QAService) extractReferences(chunks []model.CodeChunk) []model.Reference {
	refs := make([]model.Reference, 0, len(chunks))
	for _, chunk := range chunks {
		refs = append(refs, model.Reference{
			FilePath:  chunk.FilePath,
			StartLine: chunk.StartLine,
			EndLine:   chunk.EndLine,
			Language:  chunk.Language,
		})
	}
	return refs
}

func getMemoryFilePath() string {
	path := config.GetString("memory.file_path")
	if path == "" {
		path = "data/memory.md"
	}
	return path
}

func (s *QAService) readMemory() string {
	filePath := getMemoryFilePath()

	data, err := os.ReadFile(filePath)
	if err != nil {
		dir := filepath.Dir(filePath)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filePath, []byte(""), 0644)
		return ""
	}
	return string(data)
}

func (s *QAService) updateMemory(newContent string) error {
	filePath := getMemoryFilePath()

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filePath, []byte(newContent), 0644)
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

func (s *QAService) formatReferences(chunks []model.CodeChunk) string {
	if len(chunks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString("📚 参考代码块\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	for i, chunk := range chunks {
		sb.WriteString(fmt.Sprintf("【%d】%s\n", i+1, chunk.FilePath))
		sb.WriteString(fmt.Sprintf("    行号: %d-%d\n", chunk.StartLine, chunk.EndLine))

		if chunk.SymbolName != "" {
			sb.WriteString(fmt.Sprintf("    函数: %s\n", chunk.SymbolName))
		} else {
			sb.WriteString("    函数: 匿名\n")
		}

		if chunk.ParentSymbol != "" {
			sb.WriteString(fmt.Sprintf("    类名: %s\n", chunk.ParentSymbol))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
