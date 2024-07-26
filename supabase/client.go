package supabase

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/llmgate/llmgate/internal/config"
)

const (
	keysTableName         = "keys"
	usageTableName        = "key_usages"
	testSessionsTableName = "test_sessions"
)

type KeyDetails struct {
	KeyId               string `json:"key_id"`
	Key                 string `json:"key"`
	UserId              string `json:"user_id"`
	ProjectId           string `json:"project_id"`
	KeyRateLimitPerSec  *int   `json:"key_rate_limit_per_second,omitempty"`
	UserRateLimitPerSec *int   `json:"user_rate_limit_per_second,omitempty"`
}

type KeyUsage struct {
	UsageType       string   `json:"usage_type"`
	LlmProvider     *string  `json:"llm_provider,omitempty"`
	LlmModel        *string  `json:"llm_model,omitempty"`
	TraceCustomerId *string  `json:"trace_customer_id,omitempty"`
	TraceSessionId  *string  `json:"trace_session_id,omitempty"`
	Cost            *float64 `json:"cost,omitempty"`
	InputTokens     *int64   `json:"input_tokens,omitempty"`
	OutputTokens    *int64   `json:"output_tokens,omitempty"`
	IsSuccess       *bool    `json:"is_success,omitempty"`
	KeyId           string   `json:"key_id"`
}

type TestSessionLog struct {
	TraceSessionId               string  `json:"trace_session_id"`
	LlmRequest                   string  `json:"llm_request"`
	LlmResponseSuccess           *string `json:"llm_response_success,omitempty"`
	LlmResponseError             *string `json:"llm_response_error,omitempty"`
	UserId                       string  `json:"user_id"`
	Cost                         float64 `json:"cost,omitempty"`
	TraceCustomerId              *string `json:"trace_customer_id,omitempty"`
	LlmResponseEvaluationSuccess *bool   `json:"llm_response_evaluation_success,omitempty"`
	LlmResponseEvaluationReason  *string `json:"llm_response_evaluation_reason,omitempty"`
}

type SupabaseClient struct {
	superbaseConfig config.SuperbaseConfig
	cache           *cache.Cache
}

func NewSupabaseClient(superbaseConfig config.SuperbaseConfig) *SupabaseClient {
	return &SupabaseClient{
		superbaseConfig: superbaseConfig,
		cache:           cache.New(30*time.Minute, 10*time.Minute),
	}
}

func (s *SupabaseClient) GetKeyDetails(key string) (*KeyDetails, error) {
	// Check cache first
	if cachedKeyUsage, found := s.cache.Get(key); found {
		return cachedKeyUsage.(*KeyDetails), nil
	}

	var keyDetails []KeyDetails

	hashKey := s.hash(key)

	apiURL := fmt.Sprintf("%s/rest/v1/%s?key=eq.%s",
		s.superbaseConfig.Url, keysTableName, hashKey)

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
		return nil, fmt.Errorf("failed to get key usage, status code: %d, response: %s", resp.StatusCode, bodyString)
	}

	err = json.NewDecoder(resp.Body).Decode(&keyDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(keyDetails) == 0 {
		return nil, fmt.Errorf("key not found")
	}

	keyDetails[0].Key = key

	// Store in cache
	s.cache.Set(key, &keyDetails[0], cache.DefaultExpiration)

	return &keyDetails[0], nil
}

func (s *SupabaseClient) LogKeyUsage(keyUsage *KeyUsage) error {
	apiURL := fmt.Sprintf("%s/rest/v1/%s", s.superbaseConfig.Url, usageTableName)

	jsonData, err := json.Marshal(keyUsage)
	if err != nil {
		println(err.Error())
		return fmt.Errorf("failed to marshal key usage: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		println(err.Error())
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", s.superbaseConfig.Key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.superbaseConfig.Key))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		println(err.Error())
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		println(bodyString)
		return fmt.Errorf("failed to log key usage, status code: %d, response: %s", resp.StatusCode, bodyString)
	}

	return nil
}

func (s *SupabaseClient) LogTestSession(sessionLog *TestSessionLog) error {
	apiURL := fmt.Sprintf("%s/rest/v1/%s", s.superbaseConfig.Url, testSessionsTableName)

	jsonData, err := json.Marshal(sessionLog)
	if err != nil {
		println(err.Error())
		return fmt.Errorf("failed to marshal session log: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		println(err.Error())
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", s.superbaseConfig.Key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.superbaseConfig.Key))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		println(err.Error())
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyString := string(bodyBytes)
		println(bodyString)
		return fmt.Errorf("failed to log session, status code: %d, response: %s", resp.StatusCode, bodyString)
	}

	return nil
}

func (s SupabaseClient) hash(input string) string {
	hash := sha256.New()
	hash.Write([]byte(input))
	hashedData := hash.Sum(nil)
	return fmt.Sprintf("%x", hashedData)
}
