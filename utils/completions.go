package utils

import (
	"strings"

	"github.com/sashabaranov/go-openai"

	"github.com/llmgate/llmgate/models"
)

func ToChatCompletionRequestFromPrompt(systemPrompt, userPrompt, model string, temperature float32) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		Temperature: temperature,
	}
}

func ToResponseStringFromChatCompletionResponse(openaiResponse openai.ChatCompletionResponse) string {
	return openaiResponse.Choices[0].Message.Content
}

func ValidateResult(answer string, assert models.AssetTestCase) (bool, string) {
	status := false
	statusReason := ""
	if assert.Type == "Contains" {
		if strings.Contains(answer, assert.Value) {
			status = true
		} else {
			statusReason = "Expected output to Contain " + assert.Value
		}
	}

	return status, statusReason
}
