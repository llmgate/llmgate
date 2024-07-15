package handlers

import (
	"fmt"
	"math/rand"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/llmgate/llmgate/gemini"
	"github.com/llmgate/llmgate/internal/utils"
	"github.com/llmgate/llmgate/openai"
	"github.com/llmgate/llmgate/pinecone"
	"github.com/llmgate/llmgate/superbase"
)

type LLMHandler struct {
	openaiClient    openai.OpenAIClient
	geminiClient    gemini.GeminiClient
	pineconeClient  pinecone.PineconeClient
	superbaseClient superbase.SupabaseClient
	embeddingModal  string
}

func NewLLMHandler(
	openaiClient openai.OpenAIClient,
	geminiClient gemini.GeminiClient,
	pineconeClient pinecone.PineconeClient,
	superbaseClient superbase.SupabaseClient,
	embeddingModal string) *LLMHandler {
	return &LLMHandler{
		openaiClient:    openaiClient,
		geminiClient:    geminiClient,
		pineconeClient:  pineconeClient,
		superbaseClient: superbaseClient,
		embeddingModal:  embeddingModal,
	}
}

func (h *LLMHandler) LLMRequest(c *gin.Context) {
	projectName := c.Param("projectName")
	postfixUrl := c.Param("postfixUrl")
	if projectName == "" || postfixUrl == "" {
		utils.ProcessGenericBadRequest(c)
		return
	}

	endpointDetails, err := h.getEndpointDetails(projectName, postfixUrl)
	if err != nil {
		utils.ProcessGenericBadRequest(c)
		return
	}

	var openaiRequest openai.CompletionsPayload
	if err := c.ShouldBindJSON(&openaiRequest); err != nil {
		utils.ProcessGenericBadRequest(c)
		return
	}

	ragContextsStr, err := h.getRagContexts(&openaiRequest, endpointDetails.EndpointId)
	if err != nil {
		utils.ProcessGenericInternalError(c)
		return
	}

	updateOpenAIRequestData(&openaiRequest, endpointDetails, ragContextsStr)

	openAIResponse, err := h.generateOpenAIResponse(endpointDetails.LlmProvider, openaiRequest)
	if err != nil {
		utils.ProcessGenericInternalError(c)
		return
	}

	err = h.superbaseClient.LogUsage(superbase.UsageLog{
		UsageId:              uuid.New().String(),
		LlmProvider:          endpointDetails.LlmProvider,
		LlmModel:             endpointDetails.LlmModel,
		PromptTokensUsed:     openAIResponse.Usage.PromptTokens,
		CompletionTokensUsed: openAIResponse.Usage.CompletionTokens,
		EndpointId:           endpointDetails.EndpointId,
		ProjectName:          projectName,
	})
	if err != nil {
		println(err.Error())
	}

	c.JSON(http.StatusOK, openAIResponse)
}

func (h *LLMHandler) getRagContexts(openaiRequest *openai.CompletionsPayload, endpointId string) (string, error) {
	var ragContextsStr string

	ingestions, err := h.superbaseClient.GetIngestions(endpointId)
	if err != nil {
		return "", err
	}

	if len(ingestions) > 0 {
		requestContents := h.extractRequestContents(openaiRequest)
		requestEmbeddings, err := h.openaiClient.GenerateEmbeddings(
			openai.EmbeddingsPayload{
				Model: h.embeddingModal,
				Input: []string{requestContents},
			},
		)
		if err != nil {
			return "", err
		}

		ragContext, err := h.pineconeClient.Query(requestEmbeddings.Data[0].Embedding, endpointId)
		if err != nil {
			return "", err
		}
		ragContextsStr += ragContext
	}

	return ragContextsStr, nil
}

func (h *LLMHandler) extractRequestContents(openaiRequest *openai.CompletionsPayload) string {
	var requestContents string
	for _, message := range openaiRequest.Messages {
		for _, content := range message.Content {
			if content.Type == "text" {
				requestContents += content.Text
			}
		}
	}
	return requestContents
}

func (h *LLMHandler) generateOpenAIResponse(llmProvider string, openaiRequest openai.CompletionsPayload) (*openai.CompletionsResponse, error) {
	if llmProvider == "OpenAI" {
		return h.openaiClient.GenerateCompletions(openaiRequest)
	} else if llmProvider == "Gemini" {
		return h.geminiClient.GenerateCompletions(openaiRequest)
	}
	return nil, fmt.Errorf("unsupported llm provider")
}

func updateOpenAIRequestData(openaiRequest *openai.CompletionsPayload, endpointDetails *superbase.Endpoint, ragContextsStr string) {
	openaiRequest.Model = endpointDetails.LlmModel
	openaiRequest.MaxTokens = endpointDetails.LlmMaxTokens

	if endpointDetails.PrePrompt != nil {
		prePromptMessage := openai.Message{
			Role: "system",
			Content: []openai.Content{
				{Type: "text", Text: *endpointDetails.PrePrompt},
			},
		}
		openaiRequest.Messages = append([]openai.Message{prePromptMessage}, openaiRequest.Messages...)
	}

	if ragContextsStr != "" {
		postPromptMessage := openai.Message{
			Role: "system",
			Content: []openai.Content{
				{Type: "text", Text: "Relevant details that you should use. Don't use anything from outside:"},
				{Type: "text", Text: ragContextsStr},
			},
		}
		openaiRequest.Messages = append(openaiRequest.Messages, postPromptMessage)
	}

	if endpointDetails.PostPrompt != nil {
		postPromptMessage := openai.Message{
			Role: "system",
			Content: []openai.Content{
				{Type: "text", Text: *endpointDetails.PostPrompt},
			},
		}
		openaiRequest.Messages = append(openaiRequest.Messages, postPromptMessage)
	}
}

func (h *LLMHandler) getEndpointDetails(projectName, postfixUrl string) (*superbase.Endpoint, error) {
	endpointDetails, err := h.superbaseClient.GetEndpointDetails(projectName, postfixUrl)
	if err != nil {
		return nil, err
	}

	endpointOverrides, err := h.superbaseClient.GetEndpointOverrides(endpointDetails.EndpointId)
	if err != nil {
		return nil, err
	}

	if endpointOverrides == nil {
		return endpointDetails, nil
	}

	if rand.Intn(100)+1 > endpointOverrides.Distribution {
		return endpointDetails, nil
	}

	h.applyEndpointOverrides(endpointDetails, endpointOverrides)
	return endpointDetails, nil
}

func (h *LLMHandler) applyEndpointOverrides(endpointDetails *superbase.Endpoint, endpointOverrides *superbase.EndpointOverride) {
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
}
