package gemini

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	openaigo "github.com/sashabaranov/go-openai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/llmgate/llmgate/models"
	"github.com/llmgate/llmgate/utils"
)

const (
	// Gemini 1.5 Flash pricing
	gemini15FlashInputTokenCostUpTo128K  = 0.00000035
	gemini15FlashInputTokenCostOver128K  = 0.00000070
	gemini15FlashOutputTokenCostUpTo128K = 0.00000105
	gemini15FlashOutputTokenCostOver128K = 0.00000210

	// Gemini 1.5 Pro pricing
	gemini15ProInputTokenCostUpTo128K  = 0.00000350
	gemini15ProInputTokenCostOver128K  = 0.00000700
	gemini15ProOutputTokenCostUpTo128K = 0.00001050
	gemini15ProOutputTokenCostOver128K = 0.00002100

	// Gemini 1.0 Pro pricing
	gemini10ProInputTokenCost  = 0.00000050
	gemini10ProOutputTokenCost = 0.00000150
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

	setModelParameters(genModel, payload)

	prompt, err := c.convertOpenAIToGeminiPrompt(payload.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert OpenAI messages to Gemini prompt: %w", err)
	}

	geminiResponse, err := genModel.GenerateContent(ctx, prompt...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	openaiResponse := c.convertGeminiToOpenAI(payload.Model, geminiResponse)
	return c.toChatCompletionExtendedResponse(payload.Model, openaiResponse), nil
}

func (c *GeminiClient) GenerateCompletionsStream(payload openaigo.ChatCompletionRequest, apiKey string) (chan openaigo.ChatCompletionStreamResponse, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	genModel := client.GenerativeModel(payload.Model)

	setModelParameters(genModel, payload)

	prompt, err := c.convertOpenAIToGeminiPrompt(payload.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert OpenAI messages to Gemini prompt: %w", err)
	}

	iter := genModel.GenerateContentStream(ctx, prompt...)

	responseChan := make(chan openaigo.ChatCompletionStreamResponse)

	go func() {
		defer close(responseChan)
		defer client.Close()

		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				// stop stream
				return
			}

			chunks := breakIntoChunks(payload.Model, resp)
			for _, chunk := range chunks {
				responseChan <- chunk
			}
		}
	}()

	return responseChan, nil
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
func (c *GeminiClient) convertGeminiToOpenAI(model string, geminiResp *genai.GenerateContentResponse) openaigo.ChatCompletionResponse {
	choices := make([]openaigo.ChatCompletionChoice, len(geminiResp.Candidates))
	for i, candidate := range geminiResp.Candidates {
		if candidate.Content != nil {
			content := concatenateContent(candidate.Content.Parts)
			choices[i] = openaigo.ChatCompletionChoice{
				Index: int(candidate.Index),
				Message: openaigo.ChatCompletionMessage{
					Role:    mapRole(candidate.Content.Role),
					Content: content,
				},
				FinishReason: mapFinishReason(candidate.FinishReason),
			}
		}
	}

	return openaigo.ChatCompletionResponse{
		ID:      fmt.Sprintf("gemini-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: choices,
		Usage: openaigo.Usage{
			PromptTokens:     int(geminiResp.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(geminiResp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(geminiResp.UsageMetadata.TotalTokenCount),
		},
	}
}

func convertGeminiStreamToOpenAI(model string, geminiResp *genai.GenerateContentResponse) openaigo.ChatCompletionStreamResponse {
	choices := make([]openaigo.ChatCompletionStreamChoice, len(geminiResp.Candidates))
	for i, candidate := range geminiResp.Candidates {
		if candidate.Content != nil {
			content := concatenateContent(candidate.Content.Parts)
			delta := openaigo.ChatCompletionStreamChoiceDelta{}

			// Only include role in the first chunk
			if i == 0 {
				delta.Role = mapRole(candidate.Content.Role)
			}

			// Break content into smaller chunks (e.g., words)
			words := strings.Fields(content)
			if len(words) > 0 {
				delta.Content = words[0]
			}

			choices[i] = openaigo.ChatCompletionStreamChoice{
				Index:        int(candidate.Index),
				Delta:        delta,
				FinishReason: mapFinishReason(candidate.FinishReason),
			}
		}
	}

	return openaigo.ChatCompletionStreamResponse{
		ID:                fmt.Sprintf("chatcmpl-%s", utils.GenerateRandomString(29)),
		Object:            "chat.completion.chunk",
		Created:           time.Now().Unix(),
		Model:             model,
		Choices:           choices,
		SystemFingerprint: "fp_" + utils.GenerateRandomString(8),
	}
}

func (c GeminiClient) openAIRoleToGeminiRole(role string) string {
	switch strings.ToLower(role) {
	case openaigo.ChatMessageRoleUser:
		return "user"
	default:
		return "model" // Default to model role
	}
}

func (c GeminiClient) toChatCompletionExtendedResponse(model string, openAIResponse openaigo.ChatCompletionResponse) *models.ChatCompletionExtendedResponse {
	cost := calculateCost(model, openAIResponse.Usage.PromptTokens, openAIResponse.Usage.CompletionTokens)
	return &models.ChatCompletionExtendedResponse{
		ChatCompletionResponse: openAIResponse,
		Cost:                   cost,
	}
}

func setModelParameters(genModel *genai.GenerativeModel, payload openaigo.ChatCompletionRequest) {
	if payload.Temperature > 0 {
		genModel.SetTemperature(float32(payload.Temperature))
	} else {
		genModel.Temperature = nil
	}
	if payload.TopP > 0 {
		genModel.SetTopP(float32(payload.TopP))
	} else {
		genModel.TopP = nil
	}
	if payload.MaxTokens > 0 {
		genModel.SetMaxOutputTokens(int32(payload.MaxTokens))
	} else {
		genModel.MaxOutputTokens = nil
	}
}

func calculateCost(model string, promptTokens, completionTokens int) float64 {
	var inputCost, outputCost float64

	switch {
	case utils.StartsWith(model, "gemini-1.5-flash"):
		if promptTokens <= 128000 {
			inputCost, outputCost = gemini15FlashInputTokenCostUpTo128K, gemini15FlashOutputTokenCostUpTo128K
		} else {
			inputCost, outputCost = gemini15FlashInputTokenCostOver128K, gemini15FlashOutputTokenCostOver128K
		}
	case utils.StartsWith(model, "gemini-1.5-pro"):
		if promptTokens <= 128000 {
			inputCost, outputCost = gemini15ProInputTokenCostUpTo128K, gemini15ProOutputTokenCostUpTo128K
		} else {
			inputCost, outputCost = gemini15ProInputTokenCostOver128K, gemini15ProOutputTokenCostOver128K
		}
	case utils.StartsWith(model, "gemini-1.0-pro"):
		inputCost, outputCost = gemini10ProInputTokenCost, gemini10ProOutputTokenCost
	}

	return (inputCost * float64(promptTokens)) + (outputCost * float64(completionTokens))
}

func mapRole(role string) string {
	switch role {
	case "user":
		return openaigo.ChatMessageRoleUser
	case "model":
		return openaigo.ChatMessageRoleAssistant
	default:
		return "system"
	}
}

func mapFinishReason(reason genai.FinishReason) openaigo.FinishReason {
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

func concatenateContent(parts []genai.Part) string {
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

func breakIntoChunks(model string, resp *genai.GenerateContentResponse) []openaigo.ChatCompletionStreamResponse {
	var chunks []openaigo.ChatCompletionStreamResponse

	for _, candidate := range resp.Candidates {
		if candidate.Content != nil {
			content := concatenateContent(candidate.Content.Parts)
			words := strings.Fields(content)

			for i, word := range words {
				chunk := convertGeminiStreamToOpenAI(model, resp)

				// Preserve spaces by adding them between words
				if i > 0 {
					chunk.Choices[0].Delta.Content = " " + word
				} else {
					chunk.Choices[0].Delta.Content = word
				}

				// Only include role in the first chunk
				if i > 0 {
					chunk.Choices[0].Delta.Role = ""
				}

				chunks = append(chunks, chunk)
			}
		}
	}

	return chunks
}
