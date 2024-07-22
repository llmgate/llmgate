package models

import (
	openaigo "github.com/sashabaranov/go-openai"
)

type ChatCompletionExtendedResponse struct {
	ChatCompletionResponse openaigo.ChatCompletionResponse
	Cost                   float64
}
