package mockllm

import (
	"errors"
	"math/rand"
	"time"

	openaigo "github.com/sashabaranov/go-openai"
)

type MockLLMClient struct{}

func NewMockLLMClient() *MockLLMClient {
	rand.Seed(time.Now().UnixNano()) // Initialize the random seed
	return &MockLLMClient{}
}

// GenerateCompletions calls the MockLLMClient Completions API
func (c MockLLMClient) GenerateCompletions(payload openaigo.ChatCompletionRequest) (*openaigo.ChatCompletionResponse, error) {
	// Define a list of possible content strings
	contents := []openaigo.MessageContent{
		{
			Type: "text",
			Text: &openaigo.MessageText{
				Value: "This is a mock completion response.",
			},
		},
		{
			Type: "text",
			Text: &openaigo.MessageText{
				Value: "Here is another example of a response.",
			},
		},
		{
			Type: "text",
			Text: &openaigo.MessageText{
				Value: "Mock response with different content.",
			},
		},
		{
			Type: "text",
			Text: &openaigo.MessageText{
				Value: "Simulated output for testing purposes.",
			},
		},
		{
			Type: "text",
			Text: &openaigo.MessageText{
				Value: "Generated response for mock client.",
			},
		},
	}

	r := rand.Float32()
	if r < 0.01 { // Mock 1% failure case
		return nil, errors.New("mock failure: service unavailable")
	} else if r < 0.02 { // Mock 1% empty choices case
		return &openaigo.ChatCompletionResponse{
			Choices: []openaigo.ChatCompletionChoice{},
		}, nil
	} else if r < 0.03 { // Mock 1% finish_reason = "time"
		id := "mock-id"
		object := "mock-object"
		created := time.Now().Unix()
		model := "mock-model"
		role := "assistant"
		content := contents[rand.Intn(len(contents))] // Random content from the list
		index := 0
		promptTokens := int(rand.Intn(10) + 1)     // Random number between 1 and 10
		completionTokens := int(rand.Intn(10) + 1) // Random number between 1 and 10
		totalTokens := promptTokens + completionTokens

		return &openaigo.ChatCompletionResponse{
			ID:      id,
			Object:  object,
			Created: created,
			Model:   model,
			Choices: []openaigo.ChatCompletionChoice{
				{
					Index: index,
					Message: openaigo.ChatCompletionMessage{
						Role:    role,
						Content: content.Text.Value,
					},
					FinishReason: openaigo.FinishReasonContentFilter,
				},
			},
			Usage: openaigo.Usage{
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				TotalTokens:      totalTokens,
			},
		}, nil
	}

	// 97% success case
	id := "mock-id"
	object := "mock-object"
	created := time.Now().Unix()
	model := "mock-model"
	role := "assistant"
	content := contents[rand.Intn(len(contents))] // Random content from the list
	index := 0
	promptTokens := int(rand.Intn(1000) + 1)    // Random number between 1 and 1000
	completionTokens := int(rand.Intn(500) + 1) // Random number between 1 and 500
	totalTokens := promptTokens + completionTokens

	return &openaigo.ChatCompletionResponse{
		ID:      id,
		Object:  object,
		Created: created,
		Model:   model,
		Choices: []openaigo.ChatCompletionChoice{
			{
				Index: index,
				Message: openaigo.ChatCompletionMessage{
					Role:    role,
					Content: content.Text.Value,
				},
				FinishReason: openaigo.FinishReasonStop,
			},
		},
		Usage: openaigo.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		},
	}, nil
}
