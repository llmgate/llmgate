package supabase

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/llmgate/llmgate/internal/config"
)

const (
	keysTableName = "keys"
)

type KeyUsage struct {
	Key       string `json:"key"`
	UserId    string `json:"user_id"`
	ProjectId string `json:"project_id"`
}

type SupabaseClient struct {
	superbaseConfig config.SuperbaseConfig
}

func NewSupabaseClient(superbaseConfig config.SuperbaseConfig) *SupabaseClient {
	return &SupabaseClient{
		superbaseConfig: superbaseConfig,
	}
}

func (s *SupabaseClient) GetKeyUsage(key string) (*KeyUsage, error) {
	var keyUsages []KeyUsage

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

	err = json.NewDecoder(resp.Body).Decode(&keyUsages)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(keyUsages) == 0 {
		return nil, fmt.Errorf("key not found")
	}

	keyUsages[0].Key = key

	return &keyUsages[0], nil
}

func (s SupabaseClient) hash(input string) string {
	hash := sha256.New()
	hash.Write([]byte(input))
	hashedData := hash.Sum(nil)
	return fmt.Sprintf("%x", hashedData)
}
