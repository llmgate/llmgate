package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/llmgate/llmgate/internal/config"
	"github.com/llmgate/llmgate/internal/handlers"
	"github.com/llmgate/llmgate/internal/superbase"
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

	// Initialize Router
	router := gin.Default()
	// health handler
	healthHandler := handlers.NewHealthHandler()
	router.GET("/health", healthHandler.IsHealthy)
	endpointHandler := handlers.NewEndpointHandler(config.Clients.OpenAI, *superbaseClient)
	router.POST("llm/:projectName/:postfixUrl", endpointHandler.LLMRequest)
	router.Run(fmt.Sprintf(":%d", config.Server.Port))
}
