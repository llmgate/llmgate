package gemini

import (
	"context"
	"fmt"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/llmgate/llmgate/internal/config"
	"github.com/llmgate/llmgate/openai"
)

type GeminiClient struct {
	geminiConfig config.GeminiConfig
}

func NewGeminiClient(geminiConfig config.GeminiConfig) *GeminiClient {
	return &GeminiClient{
		geminiConfig: geminiConfig,
	}
}

// GenerateCompletions calls the OpenAI Completions API
func (c GeminiClient) GenerateCompletions(payload openai.CompletionsPayload) (*openai.CompletionsResponse, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(c.geminiConfig.Key))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	genModel := client.GenerativeModel(payload.Model)

	prompt := make([]genai.Part, 0)

	for _, message := range payload.Messages {
		for _, content := range message.Content {
			if content.Type == "text" {
				prompt = append(prompt, genai.Text(content.Text))
			}
			// TODO: support images
			// genai.ImageData("jpeg", *imageBytes)
		}
	}

	geminiResponse, err := genModel.GenerateContent(ctx, prompt...)
	if err != nil {
		return nil, err
	}

	openaiCompletionsResponse := convertGeminiToOpenAI(*geminiResponse)

	return &openaiCompletionsResponse, nil
}

// Convert Gemini response to OpenAI response
func convertGeminiToOpenAI(geminiResp genai.GenerateContentResponse) openai.CompletionsResponse {
	created := time.Now().Unix()
	objectType := "text_completion"

	openAIResp := openai.CompletionsResponse{
		ID:      nil, // Assuming no direct mapping
		Object:  &objectType,
		Created: &created,
		Model:   nil, // Assuming no direct mapping
		Choices: &[]struct {
			Index   *int `json:"index,omitempty"`
			Message *struct {
				Role    *string `json:"role,omitempty"`
				Content *string `json:"content,omitempty"`
			} `json:"message,omitempty"`
			FinishReason *string `json:"finish_reason,omitempty"`
		}{},
		Usage: &struct {
			PromptTokens     *int32 `json:"prompt_tokens,omitempty"`
			CompletionTokens *int32 `json:"completion_tokens,omitempty"`
			TotalTokens      *int32 `json:"total_tokens,omitempty"`
		}{
			PromptTokens:     &geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: &geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      &geminiResp.UsageMetadata.TotalTokenCount,
		},
	}

	for _, candidate := range geminiResp.Candidates {
		index := int(candidate.Index)
		role := candidate.Content.Role
		finishReason := mapFinishReason(candidate.FinishReason)
		for _, part := range candidate.Content.Parts {
			content := fmt.Sprintf("%s", part)
			choice := struct {
				Index   *int `json:"index,omitempty"`
				Message *struct {
					Role    *string `json:"role,omitempty"`
					Content *string `json:"content,omitempty"`
				} `json:"message,omitempty"`
				FinishReason *string `json:"finish_reason,omitempty"`
			}{
				Index: &index,
				Message: &struct {
					Role    *string `json:"role,omitempty"`
					Content *string `json:"content,omitempty"`
				}{
					Role:    &role,
					Content: &content,
				},
				FinishReason: &finishReason,
			}

			*openAIResp.Choices = append(*openAIResp.Choices, choice)
		}
	}

	return openAIResp
}

func mapFinishReason(reason genai.FinishReason) string {
	switch reason {
	case genai.FinishReasonStop:
		return "stop"
	case genai.FinishReasonMaxTokens:
		return "max_tokens"
	case genai.FinishReasonSafety:
		return "safety"
	case genai.FinishReasonRecitation:
		return "recitation"
	default:
		return "other"
	}
}
