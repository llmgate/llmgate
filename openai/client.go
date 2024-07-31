package openai

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/llmgate/llmgate/models"
	"github.com/llmgate/llmgate/utils"
	openaigo "github.com/sashabaranov/go-openai"
)

const (
	gpt4ominiInputTokenCost  = 0.00000015
	gpt4ominiOutputTokenCost = 0.0000006
	gpt4oInputTokenCost      = 0.000005
	gpt4oOutputTokenCost     = 0.000015
)

type OpenAIClient struct {
}

func NewOpenAIClient() *OpenAIClient {
	return &OpenAIClient{}
}

// GenerateCompletions calls the OpenAI Completions API
func (c OpenAIClient) GenerateCompletions(payload openaigo.ChatCompletionRequest, apiKey string) (*models.ChatCompletionExtendedResponse, error) {
	client := openaigo.NewClient(apiKey)
	response, err := client.CreateChatCompletion(
		context.Background(),
		payload,
	)
	if err != nil {
		return nil, err
	}

	return c.toChatCompletionExtendedResponse(payload.Model, response), nil
}

// GenerateCompletions calls the OpenAI Completions API
func (c OpenAIClient) GenerateCompletionsStream(payload openaigo.ChatCompletionRequest, apiKey string) (chan openaigo.ChatCompletionStreamResponse, chan models.StreamMetrics, error) {
	client := openaigo.NewClient(apiKey)
	responseChan := make(chan openaigo.ChatCompletionStreamResponse)
	metricsChan := make(chan models.StreamMetrics, 1) // Buffer of 1 to prevent blocking

	go func() {
		startTime := time.Now()
		totalInputTokens := 0
		totalOutputTokens := 0

		// Estimate input tokens
		for _, msg := range payload.Messages {
			totalInputTokens += len(strings.Fields(msg.Content))
		}

		stream, err := client.CreateChatCompletionStream(
			context.Background(),
			payload,
		)
		if err != nil {
			metricsChan <- models.StreamMetrics{Error: err}
			close(responseChan)
			close(metricsChan)
			return
		}
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				metricsChan <- models.StreamMetrics{Error: err}
				close(responseChan)
				close(metricsChan)
				return
			}

			responseChan <- response

			// Count output tokens in the response
			if response.Choices != nil && len(response.Choices) > 0 {
				totalOutputTokens += len(strings.Fields(response.Choices[0].Delta.Content))
			}
		}

		close(responseChan) // Close responseChan after all responses are sent

		latency := time.Since(startTime)
		cost := calculateCost(payload.Model, totalInputTokens, totalOutputTokens)

		metricsChan <- models.StreamMetrics{
			Latency:           latency,
			TotalInputTokens:  totalInputTokens,
			TotalOutputTokens: totalOutputTokens,
			Cost:              cost,
		}

		close(metricsChan) // Close metricsChan after sending metrics
	}()

	return responseChan, metricsChan, nil
}

func (c OpenAIClient) toChatCompletionExtendedResponse(model string, openAIResponse openaigo.ChatCompletionResponse) *models.ChatCompletionExtendedResponse {
	cost := calculateCost(model, openAIResponse.Usage.PromptTokens, openAIResponse.Usage.CompletionTokens)
	return &models.ChatCompletionExtendedResponse{
		ChatCompletionResponse: openAIResponse,
		Cost:                   cost,
	}
}

func calculateCost(model string, inputTokens, outputTokens int) float64 {
	var inputCost, outputCost float64

	if utils.StartsWith(model, "gpt-4o-mini") {
		inputCost, outputCost = gpt4ominiInputTokenCost, gpt4ominiOutputTokenCost
	} else if utils.StartsWith(model, "gpt-4o") {
		inputCost, outputCost = gpt4oInputTokenCost, gpt4oOutputTokenCost
	}

	return (inputCost * float64(inputTokens)) + (outputCost * float64(outputTokens))
}
