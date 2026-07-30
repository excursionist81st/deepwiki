package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type Client struct {
	client         *openai.Client
	alibabaAPIKey  string
	alibabaBaseURL string
}

type Message struct {
	Role    string
	Content string
}

func NewClient(apiKey, baseURL string) *Client {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)
	return &Client{client: &client}
}

func NewClientWithAlibaba(apiKey, baseURL, alibabaKey, alibabaURL string) *Client {
	c := NewClient(apiKey, baseURL)
	c.alibabaAPIKey = alibabaKey
	c.alibabaBaseURL = alibabaURL
	return c
}

func (c *Client) ChatStream(messages []Message, onChunk func(chunk string)) error {
	ctx := context.Background()

	chatMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	for i, msg := range messages {
		switch msg.Role {
		case "system":
			chatMessages[i] = openai.SystemMessage(msg.Content)
		case "user":
			chatMessages[i] = openai.UserMessage(msg.Content)
		case "assistant":
			chatMessages[i] = openai.AssistantMessage(msg.Content)
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:       openai.ChatModel("deepseek-v4-flash"),
		Messages:    chatMessages,
		Temperature: openai.Opt(0.0),
	}

	stream := c.client.Chat.Completions.NewStreaming(ctx, params)

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onChunk(chunk.Choices[0].Delta.Content)
		}
	}

	return stream.Err()
}

func (c *Client) Embedding(text string) ([]float64, error) {
	if c.alibabaAPIKey == "" || c.alibabaBaseURL == "" {
		return nil, nil
	}

	reqBody := map[string]interface{}{
		"model": "text-embedding-v2",
		"input": map[string]interface{}{
			"texts": []string{text},
		},
		"parameters": map[string]interface{}{
			"text_type": "document",
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.alibabaBaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.alibabaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Output struct {
			Embeddings []struct {
				Embedding []float64 `json:"embedding"`
			} `json:"embeddings"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != "" && result.Code != "Success" {
		return nil, nil
	}

	if len(result.Output.Embeddings) == 0 {
		return nil, nil
	}

	return result.Output.Embeddings[0].Embedding, nil
}

func (c *Client) EmbeddingBatch(texts []string) ([][]float64, error) {
	if c.alibabaAPIKey == "" || c.alibabaBaseURL == "" {
		return nil, nil
	}

	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := map[string]interface{}{
		"model": "text-embedding-v2",
		"input": map[string]interface{}{
			"texts": texts,
		},
		"parameters": map[string]interface{}{
			"text_type": "document",
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.alibabaBaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.alibabaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Output struct {
			Embeddings []struct {
				Embedding []float64 `json:"embedding"`
			} `json:"embeddings"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if result.Code != "" && result.Code != "Success" {
		return nil, nil
	}

	embeddings := make([][]float64, len(result.Output.Embeddings))
	for i, emb := range result.Output.Embeddings {
		embeddings[i] = emb.Embedding
	}

	return embeddings, nil
}
