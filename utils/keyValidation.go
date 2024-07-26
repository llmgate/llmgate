package utils

import "github.com/llmgate/llmgate/supabase"

func ValidateLLMGateKey(key string, supabaseClient supabase.SupabaseClient) *supabase.KeyDetails {
	if !StartsWith(key, "llmgate") {
		return nil
	}

	keyDetails, err := supabaseClient.GetKeyDetails(key)
	if err != nil {
		return nil
	}

	return keyDetails
}
