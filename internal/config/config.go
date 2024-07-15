package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server  ServerConfig
	Clients ClientsConfig
}

type ServerConfig struct {
	Port int
}

type ClientsConfig struct {
	OpenAI    OpenAIConfig
	Superbase SuperbaseConfig
	Pinecone  PineconeConfig
}

type GeminiConfig struct {
	Key   string
	Model string
}

type OpenAIConfig struct {
	Key            string
	EmbeddingModal string
}

type SuperbaseConfig struct {
	Url                        string
	Key                        string
	EndpointsTableName         string
	EndpointOverridesTableName string
	IngestionsTableName        string
	UsageLogsTableName         string
}

type PineconeConfig struct {
	Key       string
	IndexHost string
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
