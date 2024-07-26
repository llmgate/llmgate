package testvisualizationutil

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/llmgate/llmgate/supabase"
)

func ProcessVisualization(c *gin.Context, logs []supabase.TestSessionLog) {
	// convert logs to map
	processedData := processLogs(logs)
	jsonData, err := json.Marshal(processedData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	tmpl, err := template.ParseFiles(filepath.Join(dir, "visualization.html"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tmpl.Execute(c.Writer, gin.H{"Data": string(jsonData)})

}

func processLogs(logs []supabase.TestSessionLog) []map[string]interface{} {
	var processedData []map[string]interface{}
	var baseTime time.Time

	if len(logs) > 0 {
		baseTime = *logs[0].CreatedAt
	}

	for _, log := range logs {
		timeDiff := log.CreatedAt.Sub(baseTime).Milliseconds()

		entry := map[string]interface{}{
			"time":     timeDiff,
			"request":  log.LlmRequest,
			"response": log.LlmResponseSuccess,
			"error":    log.LlmResponseError,
			"cost":     log.Cost,
		}
		processedData = append(processedData, entry)
	}

	return processedData
}
