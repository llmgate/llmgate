package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	openaigo "github.com/sashabaranov/go-openai"

	"github.com/llmgate/llmgate/gemini"
	"github.com/llmgate/llmgate/mockllm"
	"github.com/llmgate/llmgate/models"
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

	var openaiRequest openaigo.ChatCompletionRequest
	if err := c.ShouldBindJSON(&openaiRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	extendedResponse, err := h.generateOpenAIResponse(llmProvider, openaiRequest, apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if extendedResponse.Cost > 0 {
		c.Header("LLM-Cost", fmt.Sprintf("$%f", extendedResponse.Cost))
	}

	c.JSON(http.StatusOK, extendedResponse.ChatCompletionResponse)
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
	openaiRequest openaigo.ChatCompletionRequest,
	apiKey string) (*models.ChatCompletionExtendedResponse, error) {
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
