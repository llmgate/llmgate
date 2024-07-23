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
	"github.com/llmgate/llmgate/supabase"
	"github.com/llmgate/llmgate/utils"
)

const (
	OpenAILLMProvider = "OpenAI"
	GeminiLLMProvider = "Gemini"
	MockLLMProvider   = "Mock"

	providerQueryKey       = "provider"
	keyHeaderKey           = "key"
	traceCustomerHeaderKey = "llmgate-trace-customer-id"
	sessionIdHeaderKey     = "llmgate-session-id"
	costHeaderResponseKey  = "LLM-Cost"
)

type LLMHandler struct {
	openaiClient   openai.OpenAIClient
	geminiClient   gemini.GeminiClient
	mockllmClient  mockllm.MockLLMClient
	supabaseClient supabase.SupabaseClient
}

func NewLLMHandler(
	openaiClient openai.OpenAIClient,
	geminiClient gemini.GeminiClient,
	mockllmClient mockllm.MockLLMClient,
	supabaseClient supabase.SupabaseClient) *LLMHandler {
	return &LLMHandler{
		openaiClient:   openaiClient,
		geminiClient:   geminiClient,
		mockllmClient:  mockllmClient,
		supabaseClient: supabaseClient,
	}
}

func (h *LLMHandler) ProcessCompletions(c *gin.Context) {
	llmProvider, _ := c.GetQuery(providerQueryKey)
	if llmProvider == "" {
		// default
		llmProvider = OpenAILLMProvider
	} else if !isValidProvider(llmProvider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid llm provider"})
		return
	}

	apiKey := c.GetHeader(keyHeaderKey)
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide api key in your header"})
		return
	}

	if utils.StartsWith(apiKey, "llmgate") {
		keyUsage := h.validateLLMGateKey(apiKey)
		if keyUsage == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "please provide a valid llmgate api key in your header"})
			return
		}
		apiKey = h.getKeyForProvider(keyUsage.UserId, llmProvider)
		if apiKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": llmProvider + " api key not configured for llmgate key"})
			return
		}
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
		c.Header(costHeaderResponseKey, fmt.Sprintf("$%f", extendedResponse.Cost))
	}

	c.JSON(http.StatusOK, extendedResponse.ChatCompletionResponse)
}

func (h *LLMHandler) TestCompletions(c *gin.Context) {
	apiKey := c.GetHeader(keyHeaderKey)
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide api key in your header"})
	}

	keyUsage := h.validateLLMGateKey(apiKey)
	if keyUsage == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide a valid llmgate api key in your header"})
	}

	var testCompletionsRequest models.TestCompletionsRequest
	if err := c.ShouldBindJSON(&testCompletionsRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	testCaseResults := make([]models.TestCaseResult, 0)

	for _, testCase := range testCompletionsRequest.TestCases {
		testResults := make([]models.TestResult, 0)
		for _, testProvider := range testCompletionsRequest.TestProviders {
			opanaiRequest := utils.ToChatCompletionRequestFromPrompt(
				testCompletionsRequest.Prompt,
				testCase.Question,
				testProvider.Model,
				testProvider.Temperature,
			)
			apiKey := h.getKeyForProvider(keyUsage.UserId, testProvider.Provider)
			if apiKey == "" {
				// skip the test case if no api key found for the provider
				continue
			}
			openaiResponse, err := h.generateOpenAIResponse(testProvider.Provider, opanaiRequest, apiKey)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			answer := utils.ToResponseStringFromChatCompletionResponse(openaiResponse.ChatCompletionResponse)
			resultStatus, statusReason := utils.ValidateResult(answer, testCase.Assert)
			testResult := models.TestResult{
				Status:       resultStatus,
				StatusReason: statusReason,
				Answer:       answer,
				TestProvider: testProvider,
				Cost:         openaiResponse.Cost,
			}
			testResults = append(testResults, testResult)
		}
		testCaseResults = append(testCaseResults, models.TestCaseResult{
			Question:    testCase.Question,
			TestResults: testResults,
		})
	}

	testCompletionsResponse := models.TestCompletionsResponse{
		TestCaseResults: testCaseResults,
	}

	c.JSON(http.StatusOK, testCompletionsResponse)
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

func (h *LLMHandler) validateLLMGateKey(key string) *supabase.KeyUsage {
	if !utils.StartsWith(key, "llmgate") {
		return nil
	}

	keyUsage, err := h.supabaseClient.GetKeyUsage(key)
	if err != nil {
		return nil
	}

	return keyUsage
}

func (h *LLMHandler) getKeyForProvider(userId, provider string) string {
	secret, err := h.supabaseClient.GetSecret(userId, provider)
	if err != nil {
		return ""
	}
	return secret.SecretValue
}
