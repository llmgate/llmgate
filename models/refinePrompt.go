package models

type RefinePromptRequest struct {
	Prompt string `json:"prompt"`
}

type RefinePromptResponse struct {
	RefinedPrompt string   `json:"refinedPrompt"`
	Reasonings    []string `json:"reasonings"`
}
