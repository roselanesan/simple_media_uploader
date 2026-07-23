package ai

import (
	"context"
	"fmt"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type Client struct {
	client openai.Client
	model  string
}

func NewClient(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = os.Getenv("WHATSAPP_AI_BASEURL")
	}
	if model == "" {
		model = os.Getenv("WHATSAPP_AI_MODEL")
	}

	return &Client{
		client: openai.NewClient(option.WithBaseURL(baseURL)),
		model:  model,
	}
}

func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	chat, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: c.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a helpful assistant. Answer concisely in Indonesian."),
			openai.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai chat: %w", err)
	}
	if len(chat.Choices) == 0 {
		return "", nil
	}
	return chat.Choices[0].Message.Content, nil
}
