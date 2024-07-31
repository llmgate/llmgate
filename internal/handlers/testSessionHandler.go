package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/llmgate/llmgate/supabase"
	testvisualizationutil "github.com/llmgate/llmgate/testVisualizationUtil"
	"github.com/llmgate/llmgate/utils"
)

const (
	sessionIdQueryKey = "sessionId"
)

type TestSessionHandler struct {
	supabaseClient supabase.SupabaseClient
}

func NewTestSessionHandler(supabaseClient supabase.SupabaseClient) *TestSessionHandler {
	return &TestSessionHandler{
		supabaseClient: supabaseClient,
	}
}

func (h *TestSessionHandler) VisualizeTestSession(c *gin.Context) {
	sessionId, _ := c.GetQuery(sessionIdQueryKey)
	if sessionId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session id is required"})
		return
	}

	apiKey := c.GetHeader(llmgateLKeyHeaderKey)
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide api key in your header"})
	}

	keyDetails := utils.ValidateLLMGateKey(apiKey, h.supabaseClient)
	if keyDetails == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "please provide a valid llmgate api key in your header"})
	}

	testLogs, err := h.supabaseClient.GetTestSessionLogs(sessionId, keyDetails.UserId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(testLogs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session id"})
	}

	testvisualizationutil.ProcessVisualization(c, testLogs)
}
