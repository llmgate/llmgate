package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	openaigo "github.com/sashabaranov/go-openai"

	"github.com/llmgate/llmgate/gemini"
	googlemonitoring "github.com/llmgate/llmgate/googleMonitoring"
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
	openaiClient           openai.OpenAIClient
	geminiClient           gemini.GeminiClient
	mockllmClient          mockllm.MockLLMClient
	supabaseClient         supabase.SupabaseClient
	googleMonitoringClient *googlemonitoring.MonitoringClient
	llmConfigs             config.LLMConfigs
	handlerConfig          config.LLMHandlerConfig
}

func NewLLMHandler(
	openaiClient openai.OpenAIClient,
	geminiClient gemini.GeminiClient,
	mockllmClient mockllm.MockLLMClient,
	supabaseClient supabase.SupabaseClient,
	googleMonitoringClient *googlemonitoring.MonitoringClient,
	llmConfigs config.LLMConfigs,
	handlerConfig config.LLMHandlerConfig) *LLMHandler {
	return &LLMHandler{
		openaiClient:           openaiClient,
		geminiClient:           geminiClient,
		mockllmClient:          mockllmClient,
		supabaseClient:         supabaseClient,
		googleMonitoringClient: googleMonitoringClient,
		llmConfigs:             llmConfigs,
		handlerConfig:          handlerConfig,
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

	var keyUsage *supabase.KeyUsage
	if utils.StartsWith(apiKey, "llmgate") && llmProvider != MockLLMProvider {
		keyUsage = h.validateLLMGateKey(apiKey)
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

	if keyUsage != nil {
		h.logCompletion(c.Request.Context(), keyUsage.UserId, keyUsage.ProjectId, llmProvider, openaiRequest.Model,
			c.GetHeader(traceCustomerHeaderKey), c.GetHeader(sessionIdHeaderKey),
			extendedResponse.Cost)
	}

	c.JSON(http.StatusOK, extendedResponse.ChatCompletionResponse)
}

func (h *LLMHandler) logCompletion(ctx context.Context,
	userId, projectId, llmProvider, llmModel,
	traceCustomerId, traceSessionId string,
	llmCost float64) {
	if h.googleMonitoringClient == nil {
		return
	}
	labels := map[string]string{
		"userId":      userId,
		"projectId":   projectId,
		"llmProvider": llmProvider,
		"llmModel":    llmModel,
	}
	if traceCustomerId != "" {
		labels["traceCustomerId"] = traceCustomerId
	}
	if traceSessionId != "" {
		labels["traceSessionId"] = traceSessionId
	}
	// add calls count
	h.googleMonitoringClient.RecordCounter("llmCalls", labels, 1)
	// add cost count
	h.googleMonitoringClient.RecordCounter("llmCost", labels, llmCost)
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

	questionResponses, err := h.executeTests(testCompletionsRequest, testCases)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	testCompletionsResponse := models.TestCompletionsResponse{
		QuestionResponses: questionResponses,
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

func (h *LLMHandler) executeTests(testCompletionsRequest models.TestCompletionsRequest, testCases []models.TestCase) ([]models.QuestionResponse, error) {
	questionResponses := make([]models.QuestionResponse, len(testCases))
	var wg sync.WaitGroup
	errChan := make(chan error, len(testCompletionsRequest.TestProviders))

	for i, testCase := range testCases {
		wg.Add(1)
		go func(i int, testCase models.TestCase) {
			defer wg.Done()

			llmResponses := make([]models.LLMResponse, len(testCompletionsRequest.TestProviders))
			var llmWg sync.WaitGroup

			for j, testProvider := range testCompletionsRequest.TestProviders {
				llmWg.Add(1)
				go func(j int, testProvider models.TestProvider) {
					defer llmWg.Done()

					apiKey := h.getKeyForProvider(testProvider.Provider)
					if apiKey == "" {
						errChan <- fmt.Errorf("API key not found for provider: %s", testProvider.Provider)
						return
					}

					opanaiRequest := utils.ToChatCompletionRequestFromPrompt(
						testCompletionsRequest.Prompt,
						testCase.Question,
						testProvider.Model,
						testProvider.Temperature,
					)

					openaiResponse, err := h.generateOpenAIResponse(testProvider.Provider, opanaiRequest, apiKey)
					if err != nil {
						log.Printf("Error generating OpenAI response for provider %s: %v", testProvider.Provider, err)
						llmResponses[j] = models.LLMResponse{
							Provider:     testProvider.Provider,
							Model:        testProvider.Model,
							Temperature:  testProvider.Temperature,
							Status:       false,
							StatusReason: fmt.Sprintf("Failed to generate LLM response: %v", err),
							Answer:       "",
							Cost:         openaiResponse.Cost,
						}
						return
					}

					answer := utils.ToResponseStringFromChatCompletionResponse(openaiResponse.ChatCompletionResponse)
					resultStatus, statusReason := utils.ValidateResult(answer, testCase.Assert)
					if answer == "" {
						statusReason = "LLM request was successful but it returned empty response"
						resultStatus = false
					}
					llmResponses[j] = models.LLMResponse{
						Provider:     testProvider.Provider,
						Model:        testProvider.Model,
						Temperature:  testProvider.Temperature,
						Status:       resultStatus,
						StatusReason: statusReason,
						Answer:       answer,
						Cost:         openaiResponse.Cost,
					}
				}(j, testProvider)
			}

			llmWg.Wait()

			questionResponses[i] = models.QuestionResponse{
				Question:     testCase.Question,
				LLMResponses: llmResponses,
			}
		}(i, testCase)
	}

	wg.Wait()
	close(errChan)

	// Check for API key errors
	for err := range errChan {
		if err != nil {
			return nil, err // Return the first error encountered
		}
	}

	return questionResponses, nil
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
	openaiRequest := utils.GetChatCompletionRequestForTestCases(userRoleDetails, h.handlerConfig.CompletionTestModel, h.handlerConfig.CompletionTestTemperature)
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
