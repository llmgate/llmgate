package models

import (
	openaigo "github.com/sashabaranov/go-openai"
)

type ChatCompletionExtendedResponse struct {
	ChatCompletionResponse openaigo.ChatCompletionResponse
	Cost                   float64
}

type TestCompletionsRequest struct {
	Prompt        string         `json:"prompt"`
	TestCases     []TestCase     `json:"testCases"`
	TestProviders []TestProvider `json:"testProviders"`
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
	TestCaseResults []TestCaseResult `json:"testCaseResults"`
}

type TestCaseResult struct {
	Question    string       `json:"question"`
	TestResults []TestResult `json:"testResults"`
}

type TestResult struct {
	Status       bool         `json:"status"`
	StatusReason string       `json:"statusReason"`
	Answer       string       `json:"answer"`
	TestProvider TestProvider `json:"testProvider"`
	Cost         float64      `json:"cost"`
}
