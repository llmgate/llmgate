package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	endpoint = "https://api.openai.com/v1/chat/completions"
)

// Content represents content in a message
type Content struct {
	Type     string   `json:"type"`
	Text     string   `json:"text,omitempty"`
	ImageURL ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents the URL of an image
type ImageURL struct {
	URL string `json:"url"`
}

// Message represents a message in a conversation
type Message struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

// Payload represents the payload sent to the OpenAI API
type Payload struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

// Response represents the structure of the response from the OpenAI API
type Response struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// CallAPI calls the OpenAI API
func CallAPI(
	payload Payload,
	apiKey string) (*Response, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error marshalling payload: %w", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + apiKey,
	}

	request, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	for key, value := range headers {
		request.Header.Set(key, value)
	}

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer response.Body.Close()

	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var openAIResponse Response
	if err := json.Unmarshal(responseData, &openAIResponse); err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %w", err)
	}

	return &openAIResponse, nil
}
