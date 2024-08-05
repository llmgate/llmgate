package claude

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liushuangls/go-anthropic"
	"github.com/llmgate/llmgate/models"
	"github.com/llmgate/llmgate/utils"
	openaigo "github.com/sashabaranov/go-openai"
)

const (
	claude3HaikuInputTokenCost   = 0.00000025
	claude3HaikuOutputTokenCost  = 0.00000125
	claude3SonnetInputTokenCost  = 0.000003
	claude3SonnetOutputTokenCost = 0.000015
	claude3OpusInputTokenCost    = 0.000015
	claude3OpusOutputTokenCost   = 0.000075
)

type ClaudeClient struct {
}

func NewClaudeClient() *ClaudeClient {
	return &ClaudeClient{}
}

func (c *ClaudeClient) GenerateCompletions(payload openaigo.ChatCompletionRequest, apiKey string) (*models.ChatCompletionExtendedResponse, error) {
	ctx := context.Background()

	client := anthropic.NewClient(apiKey)

	request := anthropic.MessagesRequest{
		Model:    payload.Model,
		System:   getSystemPrompt(payload),
		Messages: convertOpenAIToClaudeMessages(payload.Messages),
	}

	if payload.MaxTokens > 0 {
		request.MaxTokens = payload.MaxTokens
	}

	if payload.TopP > 0 {
		request.TopP = &payload.TopP
	}

	resp, err := client.CreateMessages(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	openAIResp := convertClaudeToOpenAI(payload.Model, resp)
	return c.toChatCompletionExtendedResponse(payload.Model, openAIResp), nil
}

func (c *ClaudeClient) GenerateCompletionsStream(payload openaigo.ChatCompletionRequest, apiKey string) (chan openaigo.ChatCompletionStreamResponse, chan models.StreamMetrics, error) {
	ctx := context.Background()

	client := anthropic.NewClient(apiKey)

	messages := convertOpenAIToClaudeMessages(payload.Messages)

	responseChan := make(chan openaigo.ChatCompletionStreamResponse)
	metricsChan := make(chan models.StreamMetrics, 1)

	startTime := time.Now()
	var totalInputTokens, totalOutputTokens int
	var currentContent strings.Builder

	streamReq := anthropic.MessagesStreamRequest{
		MessagesRequest: anthropic.MessagesRequest{
			Model:     payload.Model,
			MaxTokens: payload.MaxTokens,
			Messages:  messages,
			System:    getSystemPrompt(payload),
		},
		OnError: func(err anthropic.ErrorResponse) {
			metricsChan <- models.StreamMetrics{Error: fmt.Errorf("stream error: %v", err)}
		},
		OnContentBlockDelta: func(data anthropic.MessagesEventContentBlockDeltaData) {
			currentContent.WriteString(data.Delta.Text)
			totalOutputTokens += len(strings.Fields(data.Delta.Text))

			openAIResp := openaigo.ChatCompletionStreamResponse{
				ID:      fmt.Sprintf("%s-%d", data.Type, data.Index),
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   payload.Model,
				Choices: []openaigo.ChatCompletionStreamChoice{
					{
						Index: 0,
						Delta: openaigo.ChatCompletionStreamChoiceDelta{
							Role:    "assistant",
							Content: data.Delta.Text,
						},
						FinishReason: openaigo.FinishReasonNull,
					},
				},
			}
			responseChan <- openAIResp
		},
		OnMessageStop: func(data anthropic.MessagesEventMessageStopData) {
			// Approximate input tokens (you may need a proper tokenizer for accuracy)
			totalInputTokens = countApproximateTokens(payload.Messages)

			latency := time.Since(startTime)
			cost := calculateCost(payload.Model, totalInputTokens, totalOutputTokens)

			metricsChan <- models.StreamMetrics{
				Latency:           latency,
				TotalInputTokens:  totalInputTokens,
				TotalOutputTokens: totalOutputTokens,
				Cost:              cost,
			}

			close(responseChan)
			close(metricsChan)
		},
	}

	go func() {
		_, err := client.CreateMessagesStream(ctx, streamReq)
		if err != nil {
			metricsChan <- models.StreamMetrics{Error: fmt.Errorf("failed to create message stream: %w", err)}
			close(responseChan)
			close(metricsChan)
		}
	}()

	return responseChan, metricsChan, nil
}

func convertClaudeToOpenAI(model string, claudeResp anthropic.MessagesResponse) openaigo.ChatCompletionResponse {
	var openaiChoices []openaigo.ChatCompletionChoice
	for _, content := range claudeResp.Content {
		openaiChoices = append(openaiChoices, openaigo.ChatCompletionChoice{
			Index: 0,
			Message: openaigo.ChatCompletionMessage{
				Role:    openaigo.ChatMessageRoleAssistant,
				Content: content.Text,
			},
			FinishReason: openaigo.FinishReasonStop,
		})
	}
	return openaigo.ChatCompletionResponse{
		ID:      claudeResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: openaiChoices,
		Usage: openaigo.Usage{
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
	}
}

func convertOpenAIToClaudeMessages(messages []openaigo.ChatCompletionMessage) []anthropic.Message {
	claudeMessages := make([]anthropic.Message, 0)
	for _, msg := range messages {
		if msg.Role != "system" && msg.Content != "" {
			// claude supprots system role only on top as a indipendent param
			claudeMessages = append(claudeMessages, anthropic.Message{
				Role: convertRole(msg.Role),
				Content: []anthropic.MessageContent{
					{
						Type: "text",
						Text: &msg.Content,
					},
				},
			})
		}
	}
	return claudeMessages
}

func convertRole(role string) string {
	switch role {
	case openaigo.ChatMessageRoleUser:
		return "user"
	case openaigo.ChatMessageRoleAssistant:
		return "assistant"
	default:
		return "system" // Default to user for system messages
	}
}

func getSystemPrompt(payload openaigo.ChatCompletionRequest) string {
	for _, msg := range payload.Messages {
		if msg.Role == "system" {
			return msg.Content
		}
	}
	return ""
}

func (c *ClaudeClient) toChatCompletionExtendedResponse(model string, openAIResponse openaigo.ChatCompletionResponse) *models.ChatCompletionExtendedResponse {
	cost := calculateCost(model, openAIResponse.Usage.PromptTokens, openAIResponse.Usage.CompletionTokens)
	return &models.ChatCompletionExtendedResponse{
		ChatCompletionResponse: openAIResponse,
		Cost:                   cost,
	}
}

func calculateCost(model string, promptTokens, completionTokens int) float64 {
	var inputCost, outputCost float64

	switch {
	case utils.StartsWith(model, "claude-3-5-sonnet"):
		inputCost, outputCost = claude3SonnetInputTokenCost, claude3SonnetOutputTokenCost
	case utils.StartsWith(model, "claude-3-opus"):
		inputCost, outputCost = claude3OpusInputTokenCost, claude3OpusOutputTokenCost
	case utils.StartsWith(model, "claude-3-haiku"):
		inputCost, outputCost = claude3HaikuInputTokenCost, claude3HaikuOutputTokenCost
	}

	return (inputCost * float64(promptTokens)) + (outputCost * float64(completionTokens))
}

func countApproximateTokens(messages []openaigo.ChatCompletionMessage) int {
	tokenCount := 0
	for _, msg := range messages {
		tokenCount += len(strings.Fields(msg.Content))
	}
	return tokenCount
}
