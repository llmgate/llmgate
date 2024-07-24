package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	openaigo "github.com/sashabaranov/go-openai"

	"github.com/llmgate/llmgate/gemini"
	"github.com/llmgate/llmgate/internal/config"
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
	llmConfigs     config.LLMConfigs
	handlerConfig  config.LLMHandlerConfig
}

func NewLLMHandler(
	openaiClient openai.OpenAIClient,
	geminiClient gemini.GeminiClient,
	mockllmClient mockllm.MockLLMClient,
	supabaseClient supabase.SupabaseClient,
	llmConfigs config.LLMConfigs,
	handlerConfig config.LLMHandlerConfig) *LLMHandler {
	return &LLMHandler{
		openaiClient:   openaiClient,
		geminiClient:   geminiClient,
		mockllmClient:  mockllmClient,
		supabaseClient: supabaseClient,
		llmConfigs:     llmConfigs,
		handlerConfig:  handlerConfig,
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
		apiKey = h.getKeyForProvider(llmProvider)
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

	testCases := make([]models.TestCase, 0)
	if testCompletionsRequest.UserRoleDetails != "" {
		llmTestCases, err := h.getTestCases(testCompletionsRequest.UserRoleDetails)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		testCases = llmTestCases
	}

	for _, testCase := range testCompletionsRequest.TestCases {
		// add user defiend test cases
		testCases = append(testCases, testCase)
	}

	testProviderResults, err := h.executeTests(testCompletionsRequest, testCases)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	testCompletionsResponse := models.TestCompletionsResponse{
		TestProviderResults: testProviderResults,
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

func (h *LLMHandler) executeTests(testCompletionsRequest models.TestCompletionsRequest, testCases []models.TestCase) ([]models.TestProviderResult, error) {
	testProviderResults := make([]models.TestProviderResult, len(testCompletionsRequest.TestProviders))
	var wg sync.WaitGroup
	errChan := make(chan error, len(testCompletionsRequest.TestProviders))

	for i, testProvider := range testCompletionsRequest.TestProviders {
		wg.Add(1)
		go func(i int, testProvider models.TestProvider) {
			defer wg.Done()

			testResults := make([]models.TestResult, 0)
			resultChan := make(chan models.TestResult)
			semaphore := make(chan struct{}, 4) // Limit to 4 concurrent executions

			apiKey := h.getKeyForProvider(testProvider.Provider)
			if apiKey == "" {
				errChan <- fmt.Errorf("API key not found for provider: %s", testProvider.Provider)
				return
			}

			for _, testCase := range testCases {
				go func(testCase models.TestCase) {
					semaphore <- struct{}{}        // Acquire semaphore
					defer func() { <-semaphore }() // Release semaphore

					opanaiRequest := utils.ToChatCompletionRequestFromPrompt(
						testCompletionsRequest.Prompt,
						testCase.Question,
						testProvider.Model,
						testProvider.Temperature,
					)

					openaiResponse, err := h.generateOpenAIResponse(testProvider.Provider, opanaiRequest, apiKey)
					if err != nil {
						// Log the error but continue with other test cases
						log.Printf("Error generating OpenAI response for provider %s: %v", testProvider.Provider, err)
						resultChan <- models.TestResult{
							Question:     testCase.Question,
							Status:       false,
							StatusReason: fmt.Sprintf("Failed to generate response: %v", err),
						}
						return
					}

					answer := utils.ToResponseStringFromChatCompletionResponse(openaiResponse.ChatCompletionResponse)
					resultStatus, statusReason := utils.ValidateResult(answer, testCase.Assert)
					testResult := models.TestResult{
						Question:     testCase.Question,
						Status:       resultStatus,
						StatusReason: statusReason,
						Answer:       answer,
						Cost:         openaiResponse.Cost,
					}
					resultChan <- testResult
				}(testCase)
			}

			for range testCases {
				testResults = append(testResults, <-resultChan)
			}

			testProviderResults[i] = models.TestProviderResult{
				Provider:    testProvider.Provider,
				Model:       testProvider.Model,
				Temperature: testProvider.Temperature,
				TestResults: testResults,
			}
		}(i, testProvider)
	}

	wg.Wait()
	close(errChan)

	// Check for API key errors
	for err := range errChan {
		if err != nil {
			return nil, err // Return the first error encountered
		}
	}

	return testProviderResults, nil
}

func (h *LLMHandler) getKeyForProvider(provider string) string {
	switch provider {
	case "OpenAI":
		return h.llmConfigs.OpenAI.Key
	case "Gemini":
		return h.llmConfigs.Gemini.Key
	default:
		return ""
	}
}

func (h *LLMHandler) getTestCases(userRoleDetails string) ([]models.TestCase, error) {
	openaiRequest := utils.GetChatCompletionRequestForTestCases(userRoleDetails, h.handlerConfig.CompletionTestModel, h.handlerConfig.Temperature)
	openaiResponse, err := h.generateOpenAIResponse(h.handlerConfig.CompletionTestProvider, openaiRequest, h.getKeyForProvider(h.handlerConfig.CompletionTestProvider))
	if err != nil {
		return nil, err
	}

	jsonStr := openaiResponse.ChatCompletionResponse.Choices[0].Message.Content
	jsonStr = utils.CleanJSONResponse(jsonStr)

	var testCases []models.TestCase
	err = json.Unmarshal([]byte(jsonStr), &testCases)
	if err != nil {
		return nil, err
	}

	return testCases, nil
}
