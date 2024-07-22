package openai

import (
	"context"

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

func (c OpenAIClient) toChatCompletionExtendedResponse(model string, openAIResponse openaigo.ChatCompletionResponse) *models.ChatCompletionExtendedResponse {
	var cost float64
	if utils.StartsWith(model, "gpt-4o-mini") {
		cost = (gpt4ominiInputTokenCost * float64(openAIResponse.Usage.PromptTokens)) + (gpt4ominiOutputTokenCost * float64(openAIResponse.Usage.CompletionTokens))
	} else if utils.StartsWith(model, "gpt-4o") {
		cost = (gpt4oInputTokenCost * float64(openAIResponse.Usage.PromptTokens)) + (gpt4oOutputTokenCost * float64(openAIResponse.Usage.CompletionTokens))
	}
	return &models.ChatCompletionExtendedResponse{
		ChatCompletionResponse: openAIResponse,
		Cost:                   cost,
	}
}
