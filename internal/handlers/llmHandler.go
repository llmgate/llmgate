package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	openaigo "github.com/sashabaranov/go-openai"

	"github.com/llmgate/llmgate/claude"
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
	ClaudeLLMProvider = "Claude"

	providerQueryKey         = "provider"
	llmgateLKeyHeaderKey     = "key"
	llmApiHeaderKey          = "llm-api-key"
	traceCustomerHeaderKey   = "x-llmgate-trace-customer-id"
	sessionIdHeaderKey       = "x-llmgate-session-id"
	requestSourceHeaderKey   = "x-llmgate-source"
	costHeaderResponseKey    = "llm-cost"
	latencyHeaderResponseKey = "llm-latency"
)

type LLMHandler struct {
	openaiClient           openai.OpenAIClient
	geminiClient           gemini.GeminiClient
	claudeClient           claude.ClaudeClient
	mockllmClient          mockllm.MockLLMClient
	supabaseClient         supabase.SupabaseClient
	googleMonitoringClient *googlemonitoring.MonitoringClient
	llmConfigs             config.LLMConfigs
	handlerConfig          config.LLMHandlerConfig
}

func NewLLMHandler(
	openaiClient openai.OpenAIClient,
	geminiClient gemini.GeminiClient,
	claudeClient claude.ClaudeClient,
	mockllmClient mockllm.MockLLMClient,
	supabaseClient supabase.SupabaseClient,
	googleMonitoringClient *googlemonitoring.MonitoringClient,
	llmConfigs config.LLMConfigs,
	handlerConfig config.LLMHandlerConfig) *LLMHandler {
	return &LLMHandler{
		openaiClient:           openaiClient,
		geminiClient:           geminiClient,
		claudeClient:           claudeClient,
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid llm provider"})
		return
	}

	llmgateApiKey := c.GetHeader(llmgateLKeyHeaderKey)
	externalLlmApiKey := c.GetHeader(llmApiHeaderKey)
	if llmgateApiKey == "" && externalLlmApiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "please provide api key in your header"})
		return
	}

	var keyDetails *supabase.KeyDetails
	if llmProvider != MockLLMProvider && llmgateApiKey != "" {
		keyDetails = utils.ValidateLLMGateKey(llmgateApiKey, h.supabaseClient)
		if keyDetails == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "please provide a valid llmgate api key in your header"})
			return
		}
	}

	if externalLlmApiKey == "" && keyDetails != nil {
		// fetch llm api key from llmgate
		externalLlmApiKey = h.getKeyForProvider(llmProvider)
		if externalLlmApiKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": llmProvider + " api key not configured for llmgate key"})
			return
		}
	}

	var openaiRequest openaigo.ChatCompletionRequest
	if err := c.ShouldBindJSON(&openaiRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if openaiRequest.Stream {
		h.processCompletionsStreamImpl(c, llmProvider, openaiRequest, externalLlmApiKey)
		return
	}

	// non stream request

	startTime := time.Now()
	extendedResponse, err := h.generateOpenAIResponse(llmProvider, openaiRequest, externalLlmApiKey)
	latency := time.Since(startTime)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if extendedResponse.Cost > 0 {
		c.Header(costHeaderResponseKey, fmt.Sprintf("%f", extendedResponse.Cost))
	}
	c.Header(latencyHeaderResponseKey, fmt.Sprintf("%d", latency.Nanoseconds()))

	go func() {
		h.logUsageMetrics(c.Request.Context(), c.GetHeader(requestSourceHeaderKey), "completion")
	}()

	c.JSON(http.StatusOK, extendedResponse.ChatCompletionResponse)
}

func (h *LLMHandler) RefinePrompt(c *gin.Context) {
	llmProvider, _ := c.GetQuery(providerQueryKey)
	if llmProvider == "" {
		// default
		llmProvider = OpenAILLMProvider
	} else if !isValidProvider(llmProvider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid llm provider"})
		return
	}

	llmgateApiKey := c.GetHeader(llmgateLKeyHeaderKey)
	if llmgateApiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide api key in your header"})
		return
	}

	keyDetails := utils.ValidateLLMGateKey(llmgateApiKey, h.supabaseClient)
	if keyDetails == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide a valid llmgate api key in your header"})
		return
	}

	var refinePromptRequest models.RefinePromptRequest
	if err := c.ShouldBindJSON(&refinePromptRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	openaiRequest := openaigo.ChatCompletionRequest{
		Model:       "gpt-4o-mini",
		Temperature: 0,
		Messages: []openaigo.ChatCompletionMessage{
			{
				Role:    openaigo.ChatMessageRoleSystem,
				Content: h.handlerConfig.RefinePrompt,
			},
			{
				Role:    openaigo.ChatMessageRoleUser,
				Content: refinePromptRequest.Prompt,
			},
		},
	}

	response, err := h.generateOpenAIResponse(
		OpenAILLMProvider,
		openaiRequest,
		h.getKeyForProvider(OpenAILLMProvider),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	jsonStr := response.ChatCompletionResponse.Choices[0].Message.Content

	var refinePromptResponse models.RefinePromptResponse
	err = json.Unmarshal([]byte(jsonStr), &refinePromptResponse)
	if err != nil {
		println(jsonStr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	go func() {
		h.logUsageMetrics(c.Request.Context(), c.GetHeader(requestSourceHeaderKey), "refinePrompt")
	}()

	c.JSON(http.StatusOK, refinePromptResponse)
}

func (h *LLMHandler) processCompletionsStreamImpl(c *gin.Context,
	llmProvider string,
	openaiRequest openaigo.ChatCompletionRequest,
	apiKey string) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	responseChan, metricsChan, err := h.generateOpenAIStreamResponse(
		llmProvider,
		openaiRequest,
		apiKey,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for response := range responseChan {
		// Send each chunk as it comes
		c.SSEvent("", response)
		flusher.Flush()
	}

	// Send a final empty data message to signal the end of the stream
	c.SSEvent("", "[DONE]")

	metrics, ok := <-metricsChan
	if ok {
		// Metrics received
		c.SSEvent("", "[METRICS]")
		flusher.Flush()

		metricsJSON, err := json.Marshal(metrics)
		if err == nil {
			c.SSEvent("", string(metricsJSON))
			flusher.Flush()
		}
	}

	// Final signal to close the connection
	c.SSEvent("", "[CLOSE]")
	flusher.Flush()
}

func (h *LLMHandler) logUsageMetrics(ctx context.Context, requestSource, metricType string) {
	if h.googleMonitoringClient == nil {
		return
	}

	labels := map[string]string{
		"metric_source": requestSource,
		"metric_type":   metricType,
	}

	// add metrics for requests
	h.googleMonitoringClient.RecordCounter("llmgate_requests", labels, 1)
}

func isValidProvider(provider string) bool {
	switch provider {
	case OpenAILLMProvider, GeminiLLMProvider, MockLLMProvider, ClaudeLLMProvider:
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
	case ClaudeLLMProvider:
		return h.claudeClient.GenerateCompletions(openaiRequest, apiKey)
	case MockLLMProvider:
		return h.mockllmClient.GenerateCompletions(openaiRequest)
	default:
		return nil, fmt.Errorf("unsupported llm provider: %s", llmProvider)
	}
}

func (h *LLMHandler) generateOpenAIStreamResponse(
	llmProvider string,
	openaiRequest openaigo.ChatCompletionRequest,
	apiKey string) (chan openaigo.ChatCompletionStreamResponse, chan models.StreamMetrics, error) {
	switch llmProvider {
	case OpenAILLMProvider:
		return h.openaiClient.GenerateCompletionsStream(openaiRequest, apiKey)
	case GeminiLLMProvider:
		return h.geminiClient.GenerateCompletionsStream(openaiRequest, apiKey)
	case ClaudeLLMProvider:
		return h.claudeClient.GenerateCompletionsStream(openaiRequest, apiKey)
	default:
		return nil, nil, fmt.Errorf("unsupported llm provider: %s", llmProvider)
	}
}

func (h *LLMHandler) getKeyForProvider(provider string) string {
	switch provider {
	case "OpenAI":
		return h.llmConfigs.OpenAI.Key
	case "Gemini":
		return h.llmConfigs.Gemini.Key
	case "Claude":
		return h.llmConfigs.Claude.Key
	default:
		return ""
	}
}
