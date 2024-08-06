package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/llmgate/llmgate/supabase"
	"github.com/llmgate/llmgate/utils"
)

type ValidateHandler struct {
	supabaseClient supabase.SupabaseClient
}

func NewValidateHandler(
	supabaseClient supabase.SupabaseClient) *ValidateHandler {
	return &ValidateHandler{
		supabaseClient: supabaseClient,
	}
}

func (h *ValidateHandler) ValidateLLMGateKey(c *gin.Context) {
	llmgateApiKey := c.GetHeader(llmgateLKeyHeaderKey)
	if llmgateApiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "please provide api key in your header"})
		return
	}

	println(llmgateApiKey)

	keyDetails := utils.ValidateLLMGateKey(llmgateApiKey, h.supabaseClient)
	if keyDetails == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "please provide a valid llmgate api key in your header"})
		return
	}

	c.JSON(http.StatusOK, nil)
}
