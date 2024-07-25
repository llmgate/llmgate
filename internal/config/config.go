package config

import (
	"fmt"

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
