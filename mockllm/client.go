package mockllm

import (
	"errors"
	"math/rand"
	"time"

	"github.com/llmgate/llmgate/openai"
)

type MockLLMClient struct {
}

func NewMockLLMClient() *MockLLMClient {
	rand.Seed(time.Now().UnixNano()) // Initialize the random seed
	return &MockLLMClient{}
}

// GenerateCompletions calls the MockLLMClient Completions API
func (c MockLLMClient) GenerateCompletions(payload openai.CompletionsPayload) (*openai.CompletionsResponse, error) {
	// Define a list of possible content strings
	contents := []string{
		"This is a mock completion response.",
		"Here is another example of a response.",
		"Mock response with different content.",
		"Simulated output for testing purposes.",
		"Generated response for mock client.",
	}

	r := rand.Float32()
	if r < 0.01 { // Mock 1% failure case
		return nil, errors.New("mock failure: service unavailable")
	} else if r < 0.02 { // Mock 1% empty choices case
		return &openai.CompletionsResponse{
			Choices: &[]struct {
				Index   *int `json:"index,omitempty"`
				Message *struct {
					Role    *string `json:"role,omitempty"`
					Content *string `json:"content,omitempty"`
				} `json:"message,omitempty"`
				FinishReason *string `json:"finish_reason,omitempty"`
			}{},
		}, nil
	} else if r < 0.03 { // Mock 1% finish_reason = "time"
		id := "mock-id"
		object := "mock-object"
		created := time.Now().Unix()
		model := "mock-model"
		role := "assistant"
		content := contents[rand.Intn(len(contents))] // Random content from the list
		index := 0
		finishReason := "time"
		promptTokens := int32(rand.Intn(10) + 1)     // Random number between 1 and 10
		completionTokens := int32(rand.Intn(10) + 1) // Random number between 1 and 10
		totalTokens := promptTokens + completionTokens

		return &openai.CompletionsResponse{
			ID:      &id,
			Object:  &object,
			Created: &created,
			Model:   &model,
			Choices: &[]struct {
				Index   *int `json:"index,omitempty"`
				Message *struct {
					Role    *string `json:"role,omitempty"`
					Content *string `json:"content,omitempty"`
				} `json:"message,omitempty"`
				FinishReason *string `json:"finish_reason,omitempty"`
			}{
				{
					Index: &index,
					Message: &struct {
						Role    *string `json:"role,omitempty"`
						Content *string `json:"content,omitempty"`
					}{
						Role:    &role,
						Content: &content,
					},
					FinishReason: &finishReason,
				},
			},
			Usage: &struct {
				PromptTokens     *int32 `json:"prompt_tokens,omitempty"`
				CompletionTokens *int32 `json:"completion_tokens,omitempty"`
				TotalTokens      *int32 `json:"total_tokens,omitempty"`
			}{
				PromptTokens:     &promptTokens,
				CompletionTokens: &completionTokens,
				TotalTokens:      &totalTokens,
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
	finishReason := "stop"
	promptTokens := int32(rand.Intn(1000) + 1)    // Random number between 1 and 10
	completionTokens := int32(rand.Intn(500) + 1) // Random number between 1 and 10
	totalTokens := promptTokens + completionTokens

	return &openai.CompletionsResponse{
		ID:      &id,
		Object:  &object,
		Created: &created,
		Model:   &model,
		Choices: &[]struct {
			Index   *int `json:"index,omitempty"`
			Message *struct {
				Role    *string `json:"role,omitempty"`
				Content *string `json:"content,omitempty"`
			} `json:"message,omitempty"`
			FinishReason *string `json:"finish_reason,omitempty"`
		}{
			{
				Index: &index,
				Message: &struct {
					Role    *string `json:"role,omitempty"`
					Content *string `json:"content,omitempty"`
				}{
					Role:    &role,
					Content: &content,
				},
				FinishReason: &finishReason,
			},
		},
		Usage: &struct {
			PromptTokens     *int32 `json:"prompt_tokens,omitempty"`
			CompletionTokens *int32 `json:"completion_tokens,omitempty"`
			TotalTokens      *int32 `json:"total_tokens,omitempty"`
		}{
			PromptTokens:     &promptTokens,
			CompletionTokens: &completionTokens,
			TotalTokens:      &totalTokens,
		},
	}, nil
}
