package pkg

import (
	"math"
	"path/filepath"
	"sort"
	"strings"

	"deepseek_wiki/config"
	"deepseek_wiki/dao"
	"deepseek_wiki/model"
)

func CosineSimilarity(a, b []float64) float64 {
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

func ExtractKeywords(text string) []string {
	text = strings.ToLower(text)

	words := strings.Fields(text)
	keywords := make([]string, 0)

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "can": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"through": true, "during": true, "before": true, "after": true,
		"above": true, "below": true, "between": true, "under": true,
		"again": true, "further": true, "then": true, "once": true,
		"here": true, "there": true, "when": true, "where": true, "why": true,
		"how": true, "all": true, "each": true, "few": true, "more": true,
		"most": true, "other": true, "some": true, "such": true, "no": true,
		"nor": true, "not": true, "only": true, "own": true, "same": true,
		"so": true, "than": true, "too": true, "very": true,
		"的": true, "是": true, "在": true, "有": true, "和": true,
		"了": true, "这": true, "那": true, "个": true, "我": true,
	}

	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if len(word) >= 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

func VectorSearch(questionEmb []float64, chunks []model.CodeChunk) map[uint]float64 {
	scores := make(map[uint]float64)

	for i := range chunks {
		chunkEmb, err := dao.GetEmbeddingFromChunk(&chunks[i])
		if err != nil || chunkEmb == nil {
			continue
		}
		score := CosineSimilarity(questionEmb, chunkEmb)
		scores[chunks[i].ID] = score
	}

	return scores
}

func KeywordSearch(question string, chunks []model.CodeChunk) map[uint]float64 {
	keywords := ExtractKeywords(question)
	scores := make(map[uint]float64)

	for i := range chunks {
		content := strings.ToLower(chunks[i].Content)
		filePath := strings.ToLower(chunks[i].FilePath)
		symbolName := strings.ToLower(chunks[i].SymbolName)

		score := 0.0
		for _, keyword := range keywords {
			count := strings.Count(content, keyword)
			count += strings.Count(filePath, keyword) * 2
			count += strings.Count(symbolName, keyword) * 3

			if count > 0 {
				score += float64(count) / float64(len(content)+1) * 100
			}
		}

		if score > 0 {
			scores[chunks[i].ID] = score
		}
	}

	return scores
}

type ChunkScore struct {
	Chunk model.CodeChunk
	Score float64
}

func HybridSearch(vectorScores, keywordScores map[uint]float64, chunks []model.CodeChunk) []ChunkScore {
	vectorWeight := config.GetFloat("qa.vector_weight")
	if vectorWeight == 0 {
		vectorWeight = 0.7
	}

	keywordWeight := config.GetFloat("qa.keyword_weight")
	if keywordWeight == 0 {
		keywordWeight = 0.3
	}

	maxVectorScore := 0.0
	for _, score := range vectorScores {
		if score > maxVectorScore {
			maxVectorScore = score
		}
	}

	maxKeywordScore := 0.0
	for _, score := range keywordScores {
		if score > maxKeywordScore {
			maxKeywordScore = score
		}
	}

	hybridScores := make([]ChunkScore, 0, len(chunks))

	chunkMap := make(map[uint]model.CodeChunk)
	for _, chunk := range chunks {
		chunkMap[chunk.ID] = chunk
	}

	allIDs := make(map[uint]bool)
	for id := range vectorScores {
		allIDs[id] = true
	}
	for id := range keywordScores {
		allIDs[id] = true
	}

	for id := range allIDs {
		vectorScore := vectorScores[id]
		keywordScore := keywordScores[id]

		if maxVectorScore > 0 {
			vectorScore /= maxVectorScore
		}
		if maxKeywordScore > 0 {
			keywordScore /= maxKeywordScore
		}

		hybridScore := vectorWeight*vectorScore + keywordWeight*keywordScore

		if chunk, exists := chunkMap[id]; exists {
			hybridScores = append(hybridScores, ChunkScore{Chunk: chunk, Score: hybridScore})
		}
	}

	sort.Slice(hybridScores, func(i, j int) bool {
		return hybridScores[i].Score > hybridScores[j].Score
	})

	return hybridScores
}

func Rerank(question string, scores []ChunkScore, topK int) []model.CodeChunk {
	if len(scores) == 0 {
		return []model.CodeChunk{}
	}

	questionLower := strings.ToLower(question)

	for i := range scores {
		boost := 0.0

		filePath := strings.ToLower(scores[i].Chunk.FilePath)

		if strings.Contains(filePath, "test") ||
			strings.Contains(filePath, "spec") ||
			strings.Contains(filePath, "mock") {
			boost -= 0.15
		}

		if strings.Contains(filePath, "example") ||
			strings.Contains(filePath, "sample") ||
			strings.Contains(filePath, "demo") {
			boost -= 0.1
		}

		pathParts := strings.Split(filePath, "/")
		for _, part := range pathParts {
			if strings.Contains(questionLower, part) {
				boost += 0.1
			}
		}

		if scores[i].Chunk.SymbolName != "" {
			symbolLower := strings.ToLower(scores[i].Chunk.SymbolName)
			if strings.Contains(questionLower, symbolLower) {
				boost += 0.2
			} else {
				boost += 0.03
			}
		}

		if scores[i].Chunk.ParentSymbol != "" {
			parentLower := strings.ToLower(scores[i].Chunk.ParentSymbol)
			if strings.Contains(questionLower, parentLower) {
				boost += 0.15
			} else {
				boost += 0.02
			}
		}

		contentLen := len(scores[i].Chunk.Content)
		if contentLen >= 100 && contentLen <= 1500 {
			boost += 0.05
		} else if contentLen > 3000 {
			boost -= 0.1
		} else if contentLen < 50 {
			boost -= 0.05
		}

		ext := filepath.Ext(filePath)
		if ext == ".go" || ext == ".py" || ext == ".js" || ext == ".ts" || ext == ".java" {
			boost += 0.02
		}

		scores[i].Score += boost
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	result := make([]model.CodeChunk, 0, topK)
	for i := 0; i < topK && i < len(scores); i++ {
		result = append(result, scores[i].Chunk)
	}

	return result
}
