package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/llmgate/llmgate/gemini"
	"github.com/llmgate/llmgate/internal/config"
	"github.com/llmgate/llmgate/internal/handlers"
	"github.com/llmgate/llmgate/mockllm"
	"github.com/llmgate/llmgate/openai"
	"github.com/llmgate/llmgate/supabase"
)

func main() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "default"
	}

	// Initialize configuration
	config, err := config.LoadConfig(env)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize OpenAI Client
	openaiClient := openai.NewOpenAIClient()

	// Initialize Gemini Client
	geminiClient := gemini.NewGeminiClient()

	// Initialize Mock Client
	mockLLMClient := mockllm.NewMockLLMClient()

	// Supabase Client
	supabaseClient := supabase.NewSupabaseClient(config.Clients.Superbase)

	// Initialize Router
	router := gin.Default()
	// health handler
	healthHandler := handlers.NewHealthHandler()
	router.GET("/health", healthHandler.IsHealthy)
	llmHandler := handlers.NewLLMHandler(*openaiClient, *geminiClient, *mockLLMClient, *supabaseClient, config.LLM, config.Handlers.LLMHandler)
	router.POST("/completions", llmHandler.ProcessCompletions)
	router.POST("/completions/test", llmHandler.TestCompletions)

	router.Run(fmt.Sprintf(":%d", config.Server.Port))
}
