package gemini

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	openaigo "github.com/sashabaranov/go-openai"
	"google.golang.org/api/option"

	"github.com/llmgate/llmgate/models"
)

type GeminiClient struct {
}

// NewGeminiClient initializes a new GeminiClient with the provided API key.
func NewGeminiClient() *GeminiClient {
	return &GeminiClient{}
}

// GenerateCompletions calls the Gemini API using OpenAI-like request format
func (c *GeminiClient) GenerateCompletions(payload openaigo.ChatCompletionRequest, apiKey string) (*models.ChatCompletionExtendedResponse, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	genModel := client.GenerativeModel(payload.Model)

	// Set temperature
	if payload.Temperature > 0 {
		genModel.SetTemperature(float32(payload.Temperature))
	} else {
		genModel.Temperature = nil
	}

	// Set top_p if provided
	if payload.TopP > 0 {
		genModel.SetTopP(float32(payload.TopP))
	} else {
		genModel.TopP = nil
	}

	// Set max output tokens if provided
	if payload.MaxTokens > 0 {
		genModel.SetMaxOutputTokens(int32(payload.MaxTokens))
	} else {
		genModel.MaxOutputTokens = nil
	}

	prompt, err := c.convertOpenAIToGeminiPrompt(payload.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert OpenAI messages to Gemini prompt: %w", err)
	}

	geminiResponse, err := genModel.GenerateContent(ctx, prompt...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	openaiResponse := c.convertGeminiToOpenAI(geminiResponse)
	return c.toChatCompletionExtendedResponse(payload.Model, openaiResponse), nil
}

func (c *GeminiClient) convertOpenAIToGeminiPrompt(messages []openaigo.ChatCompletionMessage) ([]genai.Part, error) {
	var prompt []genai.Part
	for _, message := range messages {
		if len(message.Content) > 0 {
			prompt = append(prompt, genai.Text(message.Content))
		}
		for _, content := range message.MultiContent {
			switch content.Type {
			case "text":
				prompt = append(prompt, genai.Text(content.Text))
			case "image_url":
				img, err := c.parseImageURL(content.ImageURL.URL)
				if err != nil {
					return nil, err
				}
				prompt = append(prompt, img)
			}
		}
	}
	return prompt, nil
}

func (c *GeminiClient) parseImageURL(url string) (genai.Part, error) {
	parts := strings.Split(url, ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid data URI format")
	}

	formatParts := strings.Split(parts[0], ";")
	if len(formatParts) != 2 {
		return nil, fmt.Errorf("invalid data URI format")
	}
	format := strings.TrimPrefix(formatParts[0], "data:image/")

	imageData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 image: %w", err)
	}

	return genai.ImageData(format, imageData), nil
}

// Convert Gemini response to OpenAI response
func (c *GeminiClient) convertGeminiToOpenAI(geminiResp *genai.GenerateContentResponse) openaigo.ChatCompletionResponse {
	choices := make([]openaigo.ChatCompletionChoice, len(geminiResp.Candidates))
	for i, candidate := range geminiResp.Candidates {
		if candidate.Content != nil {
			content := c.concatenateContent(candidate.Content.Parts)
			choices[i] = openaigo.ChatCompletionChoice{
				Index: int(candidate.Index),
				Message: openaigo.ChatCompletionMessage{
					Role:    c.mapRole(candidate.Content.Role),
					Content: content,
				},
				FinishReason: c.mapFinishReason(candidate.FinishReason),
			}
		}
	}

	return openaigo.ChatCompletionResponse{
		ID:      fmt.Sprintf("gemini-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gemini", // Or a more specific model name if available
		Choices: choices,
		Usage: openaigo.Usage{
			PromptTokens:     int(geminiResp.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(geminiResp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(geminiResp.UsageMetadata.TotalTokenCount),
		},
	}
}

func (c *GeminiClient) concatenateContent(parts []genai.Part) string {
	var content strings.Builder
	for _, part := range parts {
		switch v := part.(type) {
		case genai.Text:
			content.WriteString(string(v))
			// Handle other types if necessary
		}
	}
	return content.String()
}

func (c *GeminiClient) mapRole(role string) string {
	switch role {
	case "user":
		return openaigo.ChatMessageRoleUser
	case "model":
		return openaigo.ChatMessageRoleAssistant
	default:
		return "system"
	}
}

func (c *GeminiClient) mapFinishReason(reason genai.FinishReason) openaigo.FinishReason {
	switch reason {
	case genai.FinishReasonStop:
		return openaigo.FinishReasonStop
	case genai.FinishReasonMaxTokens:
		return openaigo.FinishReasonLength
	case genai.FinishReasonSafety:
		return openaigo.FinishReasonContentFilter
	default:
		return openaigo.FinishReasonNull
	}
}

func (c *GeminiClient) toChatCompletionExtendedResponse(model string, openAIResponse openaigo.ChatCompletionResponse) *models.ChatCompletionExtendedResponse {
	return &models.ChatCompletionExtendedResponse{
		ChatCompletionResponse: openAIResponse,
		Cost:                   0, // Implement cost calculation if needed
	}
}

func openAIRoleToGeminiRole(role string) string {
	switch strings.ToLower(role) {
	case openaigo.ChatMessageRoleUser:
		return "user"
	default:
		return "model" // Default to model role
	}
}
