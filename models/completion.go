package models

import (
	openaigo "github.com/sashabaranov/go-openai"
)

type ChatCompletionExtendedResponse struct {
	ChatCompletionResponse openaigo.ChatCompletionResponse
	Cost                   float64
}

type TestCompletionsRequest struct {
	Prompt          string         `json:"prompt"`
	UserRoleDetails string         `json:"userRoleDetails"`
	TestCases       []TestCase     `json:"testCases"`
	TestProviders   []TestProvider `json:"testProviders"`
}

type TestCase struct {
	Question string        `json:"question"`
	Assert   AssetTestCase `json:"assert"`
}

type AssetTestCase struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type TestProvider struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Temperature float32 `json:"temperature"`
}

type TestCompletionsResponse struct {
	QuestionResponses []QuestionResponse `json:"questionResponses"`
}

type QuestionResponse struct {
	Question     string        `json:"question"`
	LLMResponses []LLMResponse `json:"llmResponses"`
}

type LLMResponse struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Temperature  float32 `json:"temperature"`
	Status       bool    `json:"status"`
	StatusReason string  `json:"statusReason"`
	Answer       string  `json:"answer"`
	Cost         float64 `json:"cost"`
}
