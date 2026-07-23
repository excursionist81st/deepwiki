package deepseek

import (
	"context"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type Client struct {
	client *openai.Client
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

func (c *Client) Chat(messages []Message) (string, error) {
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
		Model:    openai.ChatModel("deepseek-chat"),
		Messages: chatMessages,
	}

	resp, err := c.client.Chat.Completions.New(ctx, params,
		option.WithRequestTimeout(60*time.Second),
	)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", nil
	}

	return resp.Choices[0].Message.Content, nil
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
		Model:    openai.ChatModel("deepseek-chat"),
		Messages: chatMessages,
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
	ctx := context.Background()

	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel("deepseek-embedding"),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
	}

	resp, err := c.client.Embeddings.New(ctx, params,
		option.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return []float64{}, nil
	}

	embedding := make([]float64, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		embedding[i] = float64(v)
	}

	return embedding, nil
}
