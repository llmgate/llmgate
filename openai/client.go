package openai

import (
	"context"

	openaigo "github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
}

func NewOpenAIClient() *OpenAIClient {
	return &OpenAIClient{}
}

// GenerateCompletions calls the OpenAI Completions API
func (c OpenAIClient) GenerateCompletions(payload openaigo.ChatCompletionRequest, apiKey string) (*openaigo.ChatCompletionResponse, error) {
	client := openaigo.NewClient(apiKey)
	response, err := client.CreateChatCompletion(
		context.Background(),
		payload,
	)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
