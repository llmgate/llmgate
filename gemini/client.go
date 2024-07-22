package gemini

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
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
		for _, content := range message.MultiContent {
			if content.Type == "text" {
				prompt = append(prompt, genai.Text(content.Text))
			} else if content.Type == "image_url" {
				// Parse the data URI
				parts := strings.Split(content.ImageURL.URL, ",")
				if len(parts) != 2 {
					return nil, fmt.Errorf("invalid data URI format")
				}

				// Extract the image format
				formatParts := strings.Split(parts[0], ";")
				if len(formatParts) != 2 {
					return nil, fmt.Errorf("invalid data URI format")
				}
				format := strings.TrimPrefix(formatParts[0], "data:image/")

				// Decode the base64 data
				imageData, err := base64.StdEncoding.DecodeString(parts[1])
				if err != nil {
					return nil, fmt.Errorf("failed to decode base64 image: %v", err)
				}

				// Create the genai.ImageData
				img := genai.ImageData(format, imageData)
				prompt = append(prompt, img)
			}
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
