package gemini

import (
	"context"
	"fmt"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/llmgate/llmgate/models"
	openaigo "github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"
)

type GeminiClient struct {
}

// NewGeminiClient initializes a new GeminiClient with the provided API key.
func NewGeminiClient() *GeminiClient {
	return &GeminiClient{}
}

// GenerateCompletions calls the OpenAI Completions API
func (c GeminiClient) GenerateCompletions(payload openaigo.ChatCompletionRequest, apiKey string) (*models.ChatCompletionExtendedResponse, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	genModel := client.GenerativeModel(payload.Model)

	prompt := make([]genai.Part, 0)

	for _, message := range payload.Messages {
		for _, content := range message.Content {
			prompt = append(prompt, genai.Text(content))
		}
	}

	geminiResponse, err := genModel.GenerateContent(ctx, prompt...)
	if err != nil {
		return nil, err
	}

	openaiCompletionsResponse := convertGeminiToOpenAI(*geminiResponse)

	return c.toChatCompletionExtendedResponse(payload.Model, openaiCompletionsResponse), nil
}

// Convert Gemini response to OpenAI response with improved handling and structure
func convertGeminiToOpenAI(geminiResp genai.GenerateContentResponse) openaigo.ChatCompletionResponse {
	choices := make([]openaigo.ChatCompletionChoice, 0)
	for _, candidate := range geminiResp.Candidates {
		choice := openaigo.ChatCompletionChoice{
			Index: int(candidate.Index),
			Message: openaigo.ChatCompletionMessage{
				Role:    string(candidate.Content.Role),                // assuming Role is directly translatable
				Content: fmt.Sprintf("%v", candidate.Content.Parts[0]), // Simplified mapping, check if Parts needs to be individually converted
			},
			FinishReason: openaigo.FinishReason(mapFinishReason(candidate.FinishReason)), // Direct conversion using a mapping function
		}
		choices = append(choices, choice)
	}

	return openaigo.ChatCompletionResponse{
		ID:      "", // Assuming ID does not map directly
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   "", // Assuming Model does not map directly
		Choices: choices,
		Usage: openaigo.Usage{ // Correcting structure
			PromptTokens:     int(geminiResp.UsageMetadata.PromptTokenCount), // Ensure correct type matching
			CompletionTokens: int(geminiResp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(geminiResp.UsageMetadata.TotalTokenCount),
		},
	}
}

// Map FinishReason from genai to openaigo
func mapFinishReason(reason genai.FinishReason) string {
	switch reason {
	case genai.FinishReasonStop:
		return "stop"
	case genai.FinishReasonMaxTokens:
		return "length"
	case genai.FinishReasonSafety:
		return "content_filter"
	default:
		return "null" // Default to 'null' for unspecified cases
	}
}

func (c GeminiClient) toChatCompletionExtendedResponse(model string, openAIResponse openaigo.ChatCompletionResponse) *models.ChatCompletionExtendedResponse {
	return &models.ChatCompletionExtendedResponse{
		ChatCompletionResponse: openAIResponse,
		Cost:                   0,
	}
}
