package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/llmgate/llmgate/internal/config"
	"github.com/llmgate/llmgate/internal/handlers"
	"github.com/llmgate/llmgate/openai"
	"github.com/llmgate/llmgate/pinecone"
	"github.com/llmgate/llmgate/superbase"
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

	// Intialize Superbase
	superbaseClient := superbase.NewSupabaseClient(config.Clients.Superbase)

	// Initialize OpenAI Client
	openaiClient := openai.NewOpenAIClient(config.Clients.OpenAI)

	// Initialize Pinecone Client
	pineconeClient := pinecone.NewPineconeClient(config.Clients.Pinecone)

	// Initialize Router
	router := gin.Default()
	// health handler
	healthHandler := handlers.NewHealthHandler()
	router.GET("/health", healthHandler.IsHealthy)
	ingestionHandler :=
		handlers.NewIngestionHandler(*openaiClient, *pineconeClient, *superbaseClient, config.Clients.OpenAI.EmbeddingModal)
	router.POST("ingest/:endpointId", ingestionHandler.IngestData)
	llmHandler := handlers.NewLLMHandler(*openaiClient, *pineconeClient, *superbaseClient, config.Clients.OpenAI.EmbeddingModal)
	router.POST("llm/:projectName/:postfixUrl", llmHandler.LLMRequest)
	router.Run(fmt.Sprintf(":%d", config.Server.Port))
}
