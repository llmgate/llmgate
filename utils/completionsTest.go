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
	keywords := strings.Split(assert.Value, ",")
	totalKeywords := len(keywords)
	matchedKeywords := 0

	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(answer), strings.ToLower(strings.TrimSpace(keyword))) {
			matchedKeywords++
		}
	}

	if matchedKeywords >= (totalKeywords+1)/3 { // At least few of the keywords should be present
		status = true
	} else {
		statusReason = "Expected output to contain most of these keywords: " + assert.Value
	}

	return status, statusReason
}

func GetChatCompletionRequestForTestCases(userRoleDetails, model string, temperature float32) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: `You are a test system that tests prompts for llms. You understand what kind of user is there and accordingly you create different test cases to ask questions to llm
							Provide at least 10 questions.
							Your response should be in the json format as following:
							[{"question": "string of the question you should be asking", "assert": {"type":"Hard coded value 'Contains'", "Value": "comma seperated string of what keywords to check for contains operation"}}]
							Example:
							If a userRole is: "asking investment related qusetions"
							[{"question": "what is difference between ETFs and Stocks? which should I invest in?", "assert": {"type":"Contains", "Value": "Ownership,Diversification,Management,Trading,Risk"}}]`,
			},
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "This is userRole details to understand what kind of questions you need to ask: " + userRoleDetails,
			},
		},
		Temperature: temperature,
	}
}

func CleanJSONResponse(response string) string {
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") && strings.HasSuffix(response, "```") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
	} else if strings.HasPrefix(response, "```") && strings.HasSuffix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
	}
	return strings.TrimSpace(response)
}
