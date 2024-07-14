package handlers

import (
	"math/rand"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/llmgate/llmgate/internal/config"
	"github.com/llmgate/llmgate/internal/superbase"
	"github.com/llmgate/llmgate/openai"
)

type EndpointHandler struct {
	openAIConfig    config.OpenAIConfig
	superbaseClient superbase.SupabaseClient
}

func NewEndpointHandler(openAIConfig config.OpenAIConfig, superbaseClient superbase.SupabaseClient) *EndpointHandler {
	return &EndpointHandler{
		openAIConfig:    openAIConfig,
		superbaseClient: superbaseClient,
	}
}

func (h *EndpointHandler) LLMRequest(c *gin.Context) {
	projectName := c.Param("projectName")
	postfix_url := c.Param("postfixUrl")

	endpointDetails, err := h.getEndpointDetails(projectName, postfix_url)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}

	var openaiRequest openai.Payload
	if err := c.ShouldBindJSON(&openaiRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Update Modal
	openaiRequest.Model = endpointDetails.LlmModel

	// Update Max Tokens
	openaiRequest.MaxTokens = endpointDetails.LlmMaxTokens

	// Append PrePrompt as the first message
	if endpointDetails.PrePrompt != nil {
		prePromptMessage := openai.Message{
			Role: "system",
			Content: []openai.Content{
				{
					Type: "text",
					Text: *endpointDetails.PrePrompt,
				},
			},
		}
		openaiRequest.Messages = append([]openai.Message{prePromptMessage}, openaiRequest.Messages...)
	}

	// Append PostPrompt as the last message
	if endpointDetails.PostPrompt != nil {
		postPromptMessage := openai.Message{
			Role: "system",
			Content: []openai.Content{
				{
					Type: "text",
					Text: *endpointDetails.PostPrompt,
				},
			},
		}
		openaiRequest.Messages = append(openaiRequest.Messages, postPromptMessage)
	}

	var openAIResponse *openai.Response
	if endpointDetails.LlmProvider == "OpenAI" {
		openAIResponse, err = openai.CallAPI(
			openaiRequest,
			h.openAIConfig.Key)
	} else {
		// unsupported llm provider
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unsupported llm provider"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong. please try again!"})
		return
	}

	c.JSON(http.StatusOK, openAIResponse)
}

func (h EndpointHandler) getEndpointDetails(projectName, postfix_url string) (*superbase.Endpoint, error) {
	endpointDetails, err := h.superbaseClient.GetEndpointDetails(projectName, postfix_url)
	if err != nil {
		return nil, err
	}

	endpointOverrides, err := h.superbaseClient.GetEndpointOverrides(endpointDetails.EndpointId)
	if err != nil {
		println(err.Error())
		return nil, err
	}

	if endpointOverrides == nil {
		return endpointDetails, nil
	}

	random := rand.Intn(100) + 1

	if random > endpointOverrides.Distribution {
		return endpointDetails, nil
	}

	if endpointOverrides.LlmProvider != nil {
		endpointDetails.LlmProvider = *endpointOverrides.LlmProvider
	}

	if endpointOverrides.LlmModel != nil {
		endpointDetails.LlmModel = *endpointOverrides.LlmModel
	}

	if endpointOverrides.LlmMaxTokens > 0 {
		endpointDetails.LlmMaxTokens = endpointOverrides.LlmMaxTokens
	}

	if endpointOverrides.PrePrompt != nil {
		endpointDetails.PrePrompt = endpointOverrides.PrePrompt
	}

	if endpointOverrides.PostPrompt != nil {
		endpointDetails.PostPrompt = endpointOverrides.PostPrompt
	}

	return endpointDetails, nil
}
