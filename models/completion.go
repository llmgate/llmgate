package models

import (
	"time"

	openaigo "github.com/sashabaranov/go-openai"
)

type ChatCompletionExtendedResponse struct {
	ChatCompletionResponse openaigo.ChatCompletionResponse
	Cost                   float64
}

type StreamMetrics struct {
	Latency           time.Duration `json:"latency"`
	TotalInputTokens  int           `json:"totalInputTokens"`
	TotalOutputTokens int           `json:"totalOutputTokens"`
	Cost              float64       `json:"cost"`
	Error             error         `json:"error"`
}
