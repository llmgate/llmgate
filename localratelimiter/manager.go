package localratelimiter

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/llmgate/llmgate/supabase"
	"github.com/llmgate/llmgate/utils"
)

// RateLimiter struct to hold limiter information and related methods
type RateLimiter struct {
	userLimiters   map[string]*limiterEntry
	apiKeyLimiters map[string]*limiterEntry
	mutex          sync.Mutex
	supabaseClient supabase.SupabaseClient
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new RateLimiter instance
func NewRateLimiter(supabaseClient supabase.SupabaseClient) *RateLimiter {
	rl := &RateLimiter{
		userLimiters:   make(map[string]*limiterEntry),
		apiKeyLimiters: make(map[string]*limiterEntry),
		supabaseClient: supabaseClient,
	}
	go rl.cleanupOldLimiters()
	return rl
}

// RateLimiterMiddleware returns a gin.HandlerFunc that enforces rate limiting
func (rl *RateLimiter) RateLimiterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("key")

		if apiKey == "" || !utils.StartsWith(apiKey, "llmgate") {
			// no need to validate rate limiting
			c.Next()
			return
		}

		// fetch apikey details
		keyDetails, err := rl.supabaseClient.GetKeyDetails(apiKey)
		if err != nil {
			// no need to validate rate limiting
			c.Next()
			return
		}

		traceCustomerId := c.GetHeader("llmgate-trace-customer-id")

		useLimiter := (traceCustomerId != "" && keyDetails.UserRateLimitPerSec != nil) || (keyDetails.KeyRateLimitPerSec != nil)
		if useLimiter {
			rl.mutex.Lock()
		}
		var userLimiter *limiterEntry
		if traceCustomerId != "" && keyDetails.UserRateLimitPerSec != nil {
			userLimiter = rl.getLimiter(rl.userLimiters, "userId-"+traceCustomerId, rate.Limit(*keyDetails.UserRateLimitPerSec), *keyDetails.UserRateLimitPerSec*2)
		}
		var apiKeyLimiter *limiterEntry
		if keyDetails.KeyRateLimitPerSec != nil {
			apiKeyLimiter = rl.getLimiter(rl.apiKeyLimiters, "key-"+apiKey, rate.Limit(*keyDetails.KeyRateLimitPerSec), *keyDetails.KeyRateLimitPerSec*2)
		}
		if useLimiter {
			rl.mutex.Unlock()
		}

		if (userLimiter != nil && !userLimiter.limiter.Allow()) || (apiKeyLimiter != nil && !apiKeyLimiter.limiter.Allow()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			return
		}

		c.Next()
	}
}

// Helper function to get a rate limiter from the map, creating a new one if necessary
func (rl *RateLimiter) getLimiter(limiters map[string]*limiterEntry, key string, r rate.Limit, b int) *limiterEntry {
	if entry, exists := limiters[key]; exists {
		entry.lastSeen = time.Now()
		return entry
	}

	entry := &limiterEntry{
		limiter:  rate.NewLimiter(r, b),
		lastSeen: time.Now(),
	}
	limiters[key] = entry

	return entry
}

// Cleanup function to remove old limiters
func (rl *RateLimiter) cleanupOldLimiters() {
	for {
		time.Sleep(time.Minute)
		rl.mutex.Lock()
		for key, entry := range rl.userLimiters {
			if time.Since(entry.lastSeen) > time.Minute {
				delete(rl.userLimiters, key)
			}
		}
		for key, entry := range rl.apiKeyLimiters {
			if time.Since(entry.lastSeen) > time.Minute {
				delete(rl.apiKeyLimiters, key)
			}
		}
		rl.mutex.Unlock()
	}
}
