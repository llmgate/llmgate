package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/llmgate/llmgate/internal/config"
)

const (
	completionsEndpoint = "https://api.openai.com/v1/chat/completions"
	embeddingsEndpoint  = "https://api.openai.com/v1/embeddings"
)

type OpenAIClient struct {
	openaiConfig config.OpenAIConfig
}

func NewOpenAIClient(openaiConfig config.OpenAIConfig) *OpenAIClient {
	return &OpenAIClient{
		openaiConfig: openaiConfig,
	}
}

// Content represents content in a message
type Content struct {
	Type     string   `json:"type"`
	Text     string   `json:"text,omitempty"`
	ImageURL ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents the URL of an image
type ImageURL struct {
	URL string `json:"url"`
}

// Message represents a message in a conversation
type Message struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

// CompletionsPayload represents the payload sent to the OpenAI API
type CompletionsPayload struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

// CompletionsResponse represents the response from the OpenAI completion endpoint.
type CompletionsResponse struct {
	ID      *string `json:"id,omitempty"`
	Object  *string `json:"object,omitempty"`
	Created *int64  `json:"created,omitempty"`
	Model   *string `json:"model,omitempty"`
	Choices *[]struct {
		Index   *int `json:"index,omitempty"`
		Message *struct {
			Role    *string `json:"role,omitempty"`
			Content *string `json:"content,omitempty"`
		} `json:"message,omitempty"`
		FinishReason *string `json:"finish_reason,omitempty"`
	} `json:"choices,omitempty"`
	Usage *struct {
		PromptTokens     *int32 `json:"prompt_tokens,omitempty"`
		CompletionTokens *int32 `json:"completion_tokens,omitempty"`
		TotalTokens      *int32 `json:"total_tokens,omitempty"`
	} `json:"usage,omitempty"`
}

// GenerateCompletions calls the OpenAI Completions API
func (c OpenAIClient) GenerateCompletions(payload CompletionsPayload) (*CompletionsResponse, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshalling payload: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + c.openaiConfig.Key,
	}

	request, err := http.NewRequest("POST", completionsEndpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	for key, value := range headers {
		request.Header.Set(key, value)
	}

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer response.Body.Close()

	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var openAIResponse CompletionsResponse
	if err := json.Unmarshal(responseData, &openAIResponse); err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %w", err)
	}

	return &openAIResponse, nil
}

// EmbeddingsPayload represents the payload sent to the OpenAI Embeddings API
type EmbeddingsPayload struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingsResponse represents the structure of the response from the OpenAI Embeddings API
type EmbeddingsResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// GenerateEmbeddings calls the OpenAI Embeddings API
func (c OpenAIClient) GenerateEmbeddings(
	payload EmbeddingsPayload) (*EmbeddingsResponse, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshalling payload: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + c.openaiConfig.Key,
	}

	request, err := http.NewRequest("POST", embeddingsEndpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	for key, value := range headers {
		request.Header.Set(key, value)
	}

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer response.Body.Close()

	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var embeddingsResponse EmbeddingsResponse
	if err := json.Unmarshal(responseData, &embeddingsResponse); err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %w", err)
	}

	return &embeddingsResponse, nil
}
