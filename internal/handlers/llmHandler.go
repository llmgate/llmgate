package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/llmgate/llmgate/gemini"
	"github.com/llmgate/llmgate/mockllm"
	"github.com/llmgate/llmgate/openai"
)

const (
	OpenAILLMProvider = "OpenAI"
	GeminiLLMProvider = "Gemini"
	MockLLMProvider   = "Mock"
)

type LLMHandler struct {
	openaiClient  openai.OpenAIClient
	geminiClient  gemini.GeminiClient
	mockllmClient mockllm.MockLLMClient
}

func NewLLMHandler(
	openaiClient openai.OpenAIClient,
	geminiClient gemini.GeminiClient,
	mockllmClient mockllm.MockLLMClient) *LLMHandler {
	return &LLMHandler{
		openaiClient:  openaiClient,
		geminiClient:  geminiClient,
		mockllmClient: mockllmClient,
	}
}

func (h *LLMHandler) ProcessCompletions(c *gin.Context) {
	llmProvider, _ := c.GetQuery("provider")
	if llmProvider == "" {
		// default
		llmProvider = OpenAILLMProvider
	} else if !isValidProvider(llmProvider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid llm provider"})
		return
	}

	apiKey := c.GetHeader("key")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide api key in your header"})
	}

	var openaiRequest openai.CompletionsPayload
	if err := c.ShouldBindJSON(&openaiRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	openAIResponse, err := h.generateOpenAIResponse(llmProvider, openaiRequest, apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, openAIResponse)
}

type SimplifiedCompletionsPayload struct {
	SystemPrePrompt    *string `json:"systemPrePromp,omitempty"`
	UserPrompt         string  `json:"userPrompt"`
	SystemPostPrompt   *string `json:"systemPostPrompt,omitempty"`
	ResponseJsonSchema *string `json:"responseJsonSchema,omitempty"`
	Model              string  `json:"model"`
	Temperature        float32 `json:"temperature"`
	MaxTokens          int     `json:"max_tokens"`
}

type SimplifiedCompletionsResponse struct {
	Response string
}

func (h *LLMHandler) ProcessSimplifiedCompletions(c *gin.Context) {
	llmProvider, _ := c.GetQuery("provider")
	if llmProvider == "" {
		// default
		llmProvider = OpenAILLMProvider
	} else if !isValidProvider(llmProvider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid llm provider"})
		return
	}

	apiKey := c.GetHeader("key")
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide api key in your header"})
	}

	var simplifiedCompletionsPayload SimplifiedCompletionsPayload
	if err := c.ShouldBindJSON(&simplifiedCompletionsPayload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if simplifiedCompletionsPayload.UserPrompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userPrompt can not be empty"})
		return
	}

	// Pre
	messages := make([]openai.Message, 0)
	if simplifiedCompletionsPayload.SystemPrePrompt != nil && *simplifiedCompletionsPayload.SystemPrePrompt != "" {
		messages = append(messages, openai.Message{
			Role: "system",
			Content: []openai.Content{
				{
					Type: "text",
					Text: *simplifiedCompletionsPayload.SystemPrePrompt,
				},
			},
		})
	}
	// Prompt
	messages = append(messages, openai.Message{
		Role: "user",
		Content: []openai.Content{
			{
				Type: "text",
				Text: *&simplifiedCompletionsPayload.UserPrompt,
			},
		},
	})
	// Post
	if simplifiedCompletionsPayload.SystemPostPrompt != nil && *simplifiedCompletionsPayload.SystemPostPrompt != "" {
		messages = append(messages, openai.Message{
			Role: "system",
			Content: []openai.Content{
				{
					Type: "text",
					Text: *simplifiedCompletionsPayload.SystemPostPrompt,
				},
			},
		})
	}
	// schema
	if simplifiedCompletionsPayload.ResponseJsonSchema != nil && *simplifiedCompletionsPayload.ResponseJsonSchema != "" {
		messages = append(messages, openai.Message{
			Role: "system",
			Content: []openai.Content{
				{
					Type: "text",
					Text: "Your response should be in this format: " + *simplifiedCompletionsPayload.ResponseJsonSchema,
				},
			},
		})
	}
	openaiRequest := openai.CompletionsPayload{
		Model:       simplifiedCompletionsPayload.Model,
		Messages:    messages,
		Temperature: simplifiedCompletionsPayload.Temperature,
		MaxTokens:   simplifiedCompletionsPayload.MaxTokens,
	}

	openAIResponse, err := h.generateOpenAIResponse(llmProvider, openaiRequest, apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if openAIResponse.Choices == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "empty response"})
		return
	}

	responseStr := ""
	for _, choice := range *openAIResponse.Choices {
		responseStr += *choice.Message.Content
	}

	if responseStr == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "empty response"})
		return
	}

	if simplifiedCompletionsPayload.ResponseJsonSchema != nil {
		var jsonResponse map[string]interface{}
		err := json.Unmarshal([]byte(responseStr), &jsonResponse)
		if err != nil {
			println(responseStr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get json response from llm"})
			return
		}
		c.JSON(http.StatusOK, jsonResponse)
		return
	}

	c.String(http.StatusOK, responseStr)
}

func isValidProvider(provider string) bool {
	switch provider {
	case OpenAILLMProvider, GeminiLLMProvider, MockLLMProvider:
		return true
	default:
		return false
	}
}

func (h *LLMHandler) generateOpenAIResponse(
	llmProvider string,
	openaiRequest openai.CompletionsPayload,
	apiKey string) (*openai.CompletionsResponse, error) {
	switch llmProvider {
	case OpenAILLMProvider:
		return h.openaiClient.GenerateCompletions(openaiRequest, apiKey)
	case GeminiLLMProvider:
		return h.geminiClient.GenerateCompletions(openaiRequest, apiKey)
	case MockLLMProvider:
		return h.mockllmClient.GenerateCompletions(openaiRequest)
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", llmProvider)
	}
}
