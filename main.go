package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/llmgate/llmgate/gemini"
	googlemonitoring "github.com/llmgate/llmgate/googleMonitoring"
	vconfig "github.com/llmgate/llmgate/internal/config"
	"github.com/llmgate/llmgate/internal/handlers"
	"github.com/llmgate/llmgate/localratelimiter"
	"github.com/llmgate/llmgate/mockllm"
	"github.com/llmgate/llmgate/openai"
	"github.com/llmgate/llmgate/supabase"
)

func main() {
	var config *vconfig.Config
	var err error

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "default"
	}
	secretBucket := os.Getenv("SECRET_BUCKET")
	secretObject := os.Getenv("SECRET_OBJECT")

	if len(secretBucket) > 0 && len(secretObject) > 0 {
		// fetch from the bucket
		config, err = vconfig.LoadConfigFromGCS(secretBucket, secretObject)
	} else {
		config, err = vconfig.LoadConfig(env)
	}

	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()

	// Initialize OpenAI Client
	openaiClient := openai.NewOpenAIClient()

	// Initialize Gemini Client
	geminiClient := gemini.NewGeminiClient()

	// Initialize Mock Client
	mockLLMClient := mockllm.NewMockLLMClient()

	// Supabase Client
	supabaseClient := supabase.NewSupabaseClient(config.Clients.Superbase)

	// Google Monitoring Client
	googleMonitoringClient, err := googlemonitoring.NewMonitoringClient(ctx, config.GoogleService.ProjectId, config.GoogleService.JsonKey)
	if err != nil {
		log.Fatalf("Failed to create monitoring client: %v", err)
	}
	defer googleMonitoringClient.Close()

	// Rate Limiter
	rateLimiter := localratelimiter.NewRateLimiter(*supabaseClient)

	// Initialize Router
	router := gin.Default()
	// Local Rate Limiter
	router.Use(rateLimiter.RateLimiterMiddleware())
	// Metrics handler
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	// Health Handler
	healthHandler := handlers.NewHealthHandler()
	router.GET("/health", healthHandler.IsHealthy)
	// LLM Handler
	llmHandler := handlers.NewLLMHandler(*openaiClient, *geminiClient, *mockLLMClient, *supabaseClient, googleMonitoringClient, config.LLM, config.Handlers.LLMHandler)
	router.POST("/completions", llmHandler.ProcessCompletions)

	go func() {
		for {
			if err := googleMonitoringClient.PushMetrics(ctx); err != nil {
				log.Printf("failed to push metrics: %v", err)
			}
			time.Sleep(60 * time.Second) // Push metrics every 60 seconds
		}
	}()

	router.Run(fmt.Sprintf(":%d", config.Server.Port))
}
