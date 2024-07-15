package superbase

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/llmgate/llmgate/internal/config"
)

type Endpoint struct {
	EndpointId   string  `json:"endpoint_id"`
	PostfixUrl   string  `json:"postfix_url"`
	LlmProvider  string  `json:"llm_provider"`
	LlmModel     string  `json:"llm_model"`
	LlmMaxTokens int     `json:"llm_max_tokens"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
	PrePrompt    *string `json:"pre_prompt,omitempty"`
	PostPrompt   *string `json:"post_prompt,omitempty"`
	ProjectName  string  `json:"project_name"`
}

type EndpointOverride struct {
	EndpointOverrideId string  `json:"endpoint_override_id"`
	Distribution       int     `json:"distribution"`
	LlmProvider        *string `json:"llm_provider,omitempty"`
	LlmModel           *string `json:"llm_model,omitempty"`
	LlmMaxTokens       int     `json:"llm_max_tokens,omitempty"`
	PrePrompt          *string `json:"pre_prompt,omitempty"`
	PostPrompt         *string `json:"post_prompt,omitempty"`
	CreatedAt          string  `json:"created_at"`
	EndpointId         string  `json:"endpoint_id"`
}

type Ingestion struct {
	IngestionId string  `json:"ingestion_id"`
	ContentType string  `json:"content_type"`
	DataUrl     string  `json:"data_url"`
	CreatedAt   *string `json:"created_at,omitempty"`
	CompletedAt *string `json:"completed_at,omitempty"`
	EndpointId  string  `json:"endpoint_id"`
}

type UsageLog struct {
	UsageId              string  `json:"usage_id,omitempty"`
	LlmProvider          string  `json:"llm_provider"`
	LlmModel             string  `json:"llm_model"`
	PromptTokensUsed     *int32  `json:"prompt_tokens_used,omitempty"`
	CompletionTokensUsed *int32  `json:"completion_tokens_used,omitempty"`
	CreatedAt            *string `json:"created_at,omitempty"`
	EndpointId           string  `json:"endpoint_id"`
	ProjectName          string  `json:"project_name"`
}

type SupabaseClient struct {
	superbaseConfig config.SuperbaseConfig
}

func NewSupabaseClient(superbaseConfig config.SuperbaseConfig) *SupabaseClient {
	return &SupabaseClient{
		superbaseConfig: superbaseConfig,
	}
}

func (s *SupabaseClient) GetEndpointDetails(projectName, postfix_url string) (*Endpoint, error) {
	var endpoints []Endpoint
	apiURL := fmt.Sprintf("%s/rest/v1/%s?postfix_url=eq.%s&project_name=eq.%s",
		s.superbaseConfig.Url, s.superbaseConfig.EndpointsTableName, postfix_url, projectName)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", s.superbaseConfig.Key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.superbaseConfig.Key))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		return nil, fmt.Errorf("failed to get endpoint, status code: %d, response: %s", resp.StatusCode, bodyString)
	}

	err = json.NewDecoder(resp.Body).Decode(&endpoints)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(endpoints) == 0 {
		return nil, fmt.Errorf("endpoint not found")
	}

	return &endpoints[0], nil
}

func (s *SupabaseClient) GetEndpointOverrides(endpointId string) (*EndpointOverride, error) {
	var overrides []EndpointOverride
	apiURL := fmt.Sprintf("%s/rest/v1/%s?endpoint_id=eq.%s",
		s.superbaseConfig.Url, s.superbaseConfig.EndpointOverridesTableName, endpointId)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", s.superbaseConfig.Key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.superbaseConfig.Key))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		return nil, fmt.Errorf("failed to get endpoint override, status code: %d, response: %s", resp.StatusCode, bodyString)
	}

	err = json.NewDecoder(resp.Body).Decode(&overrides)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(overrides) == 0 {
		return nil, nil
	}

	return &overrides[0], nil
}

func (s *SupabaseClient) StoreIngestion(ingestion Ingestion) error {
	apiURL := fmt.Sprintf("%s/rest/v1/%s",
		s.superbaseConfig.Url, s.superbaseConfig.IngestionsTableName)

	data, err := json.Marshal(ingestion)
	if err != nil {
		return fmt.Errorf("failed to marshal ingestion: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", s.superbaseConfig.Key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.superbaseConfig.Key))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		return fmt.Errorf("failed to store ingestion, status code: %d, response: %s", resp.StatusCode, bodyString)
	}

	return nil
}

func (s *SupabaseClient) GetIngestions(endpointId string) ([]Ingestion, error) {
	var ingestions []Ingestion
	apiURL := fmt.Sprintf("%s/rest/v1/%s?endpoint_id=eq.%s",
		s.superbaseConfig.Url, s.superbaseConfig.IngestionsTableName, endpointId)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", s.superbaseConfig.Key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.superbaseConfig.Key))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		return nil, fmt.Errorf("failed to get ingestions, status code: %d, response: %s", resp.StatusCode, bodyString)
	}

	err = json.NewDecoder(resp.Body).Decode(&ingestions)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return ingestions, nil
}

func (s *SupabaseClient) LogUsage(usageLog UsageLog) error {
	apiURL := fmt.Sprintf("%s/rest/v1/%s",
		s.superbaseConfig.Url, s.superbaseConfig.UsageLogsTableName)

	data, err := json.Marshal(usageLog)
	if err != nil {
		return fmt.Errorf("failed to marshal usage log: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", s.superbaseConfig.Key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.superbaseConfig.Key))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		return fmt.Errorf("failed to log usage, status code: %d, response: %s", resp.StatusCode, bodyString)
	}

	return nil
}
