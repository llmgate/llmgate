package supabase

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/llmgate/llmgate/internal/config"
)

const (
	keysTableName    = "keys"
	secretsTableName = "secrets"
)

type KeyUsage struct {
	Key    string `json:"key"`
	UserId string `json:"user_id"`
}

type Secret struct {
	SecretId    string `json:"secret_id"`
	Provider    string `json:"provider"`
	UserId      string `json:"user_id"`
	SecretValue string `json:"secret_value"`
}

type SecretValue struct {
	SecretId string `json:"secret_id"`
	Value    string `json:"value"`
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

	apiURL := fmt.Sprintf("%s/rest/v1/%s?key=eq.%s",
		s.superbaseConfig.Url, keysTableName, key)

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

	return &keyUsages[0], nil
}

func (s *SupabaseClient) GetSecret(userId, provider string) (*Secret, error) {
	var secrets []Secret
	apiURL := fmt.Sprintf("%s/rest/v1/%s?user_id=eq.%s&provider=eq.%s",
		s.superbaseConfig.Url, secretsTableName, userId, provider)

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
		return nil, fmt.Errorf("failed to get secret, status code: %d, response: %s", resp.StatusCode, bodyString)
	}

	err = json.NewDecoder(resp.Body).Decode(&secrets)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(secrets) == 0 {
		return nil, fmt.Errorf("secret not found")
	}

	descryptedSecret, err := s.decrypt(secrets[0].SecretValue)
	if err != nil {
		return nil, err
	}
	secrets[0].SecretValue = descryptedSecret

	return &secrets[0], nil
}

func (s SupabaseClient) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher([]byte(s.superbaseConfig.EncrpytionKey))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s SupabaseClient) decrypt(ciphertext string) (string, error) {
	block, err := aes.NewCipher([]byte(s.superbaseConfig.EncrpytionKey))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
