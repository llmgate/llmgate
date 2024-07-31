package config

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"github.com/spf13/viper"
)

type Config struct {
	Server        ServerConfig
	GoogleService GoogleServiceConfig
	Handlers      HandlersConfig
	LLM           LLMConfigs
	Clients       ClientConfigs
}

type ServerConfig struct {
	Port int
}

type GoogleServiceConfig struct {
	ProjectId string
	JsonKey   string
}

type HandlersConfig struct {
	LLMHandler LLMHandlerConfig
}

type LLMHandlerConfig struct {
	CompletionTestProvider      string
	CompletionTestModel         string
	CompletionTestTemperature   float32
	CompletionTestParallelCount int
}

type LLMConfigs struct {
	OpenAI OpenAIConfig
	Gemini GeminiConfig
}

type OpenAIConfig struct {
	Key string
}

type GeminiConfig struct {
	Key string
}

type ClientConfigs struct {
	Superbase SuperbaseConfig
}

type SuperbaseConfig struct {
	Url string
	Key string
}

func LoadConfig(configName string) (*Config, error) {
	var config Config

	viper.SetConfigName(configName)
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	return &config, nil
}

func LoadConfigFromGCS(bucketName, objectName string) (*Config, error) {
	var config Config

	// Read the configuration file from GCS
	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	rc, err := client.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %v", err)
	}

	// Unmarshal the data into Viper
	viper.SetConfigType("yaml")
	if err := viper.ReadConfig(bytes.NewBuffer(data)); err != nil {
		return nil, fmt.Errorf("failed to read config: %v", err)
	}

	// Unmarshal the configuration into the Config struct
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %v", err)
	}

	return &config, nil
}
