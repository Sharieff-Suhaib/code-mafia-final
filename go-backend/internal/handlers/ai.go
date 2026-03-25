package handlers

import (
	"bytes"
	"code-mafia-backend/internal/database"
	"code-mafia-backend/internal/redis"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const groqAPIURL = "https://api.groq.com/openai/v1/chat/completions"
const groqModel = "openai/gpt-oss-120b"
const cacheExpiry = 24 * time.Hour

type AIHandler struct {
	repo       *database.Repository
	redis      *redis.Client
	groqAPIKey string
}

func NewAIHandler(repo *database.Repository, redisClient *redis.Client, groqAPIKey string) *AIHandler {
	return &AIHandler{repo: repo, redis: redisClient, groqAPIKey: groqAPIKey}
}

// groqRequest / groqResponse types for the Groq API
type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Messages            []groqMessage `json:"messages"`
	Model               string        `json:"model"`
	Temperature         float64       `json:"temperature"`
	MaxCompletionTokens int           `json:"max_completion_tokens"`
	TopP                float64       `json:"top_p"`
	Stream              bool          `json:"stream"`
	Stop                interface{}   `json:"stop"`
}

type groqChoice struct {
	Message groqMessage `json:"message"`
}

type groqResponse struct {
	Choices []groqChoice `json:"choices"`
}

// GenerateStarterCode godoc
// GET /api/ai/starter?challengeId=xxx
// Returns {"python":"...", "cpp":"...", "java":"...", "c":"...", "javascript":"..."}
func (h *AIHandler) GenerateStarterCode(w http.ResponseWriter, r *http.Request) {
	challengeID := r.URL.Query().Get("challengeId")
	if challengeID == "" {
		respondWithError(w, http.StatusBadRequest, "challengeId is required")
		return
	}

	cacheKey := fmt.Sprintf("ai-skel-v2:%s", challengeID)

	// --- Cache hit ---
	cached, err := h.redis.Get(cacheKey)
	if err == nil && cached != "" {
		var result map[string]string
		if json.Unmarshal([]byte(cached), &result) == nil {
			respondWithJSON(w, http.StatusOK, map[string]interface{}{
				"starter_code": result,
				"from_cache":   true,
			})
			return
		}
	}

	// --- Fetch challenge from DB ---
	challenge, err := h.repo.GetChallengeByID(challengeID)
	if err != nil || challenge == nil {
		respondWithError(w, http.StatusNotFound, "Challenge not found")
		return
	}

	// Build test case examples for the prompt
	var tcBuilder strings.Builder
	for i, tc := range challenge.TestCases {
		if i >= 3 { // limit to 3 examples to keep prompt concise
			break
		}
		tcBuilder.WriteString(fmt.Sprintf("  Input: %s\n  Expected Output: %s\n", tc.Input, tc.ExpectedOutput))
	}

	prompt := fmt.Sprintf(`You are generating SKELETON / STARTER code for a coding contest. The user will fill in the logic themselves.

Problem Title: %s
Problem Description: %s
Test Case Input Format (each line = one argument):
%s

Your task: For ALL 5 languages, generate a SINGLE FILE that contains:
1. The solution function — correct name, parameter names and types — but the body must be a STUB ONLY:
   - Python: just "    pass"
   - JavaScript: just empty braces {}
   - C++: just "    // Your code here\n    return {};" (or appropriate empty return)
   - Java: just "    // Your code here\n    return null;" (or appropriate empty return)
   - C: just "    // Your code here\n    return 0;" (or appropriate empty return)
2. A COMPLETE main function that reads stdin per the input format and calls the stub function and prints the result.

Example of what a CORRECT skeleton looks like for Python (Two Sum problem):

import sys, json
def twoSum(nums, target):
    pass  # <-- STUB ONLY, no logic
if __name__ == "__main__":
    lines = sys.stdin.read().strip().split("\n")
    nums = json.loads(lines[0])
    target = int(lines[1])
    print(twoSum(nums, target))

NOTICE: The function body is ONLY "pass". No algorithm is implemented.

Do the same for all 5 languages. DO NOT implement any algorithm. Function bodies must be stubs.

Respond ONLY with a raw JSON object (no markdown, no explanation):
{"python": "...", "cpp": "...", "java": "...", "c": "...", "javascript": "..."}`,
		challenge.Title, challenge.Description, tcBuilder.String(),
	)

	starterCode, err := h.callGroq(prompt)
	if err != nil {
		log.Printf("Groq API error for challenge %s: %v", challengeID, err)
		// Fallback: return whatever is stored in DB
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"starter_code": challenge.StarterCode,
			"from_cache":   false,
			"ai_error":     err.Error(),
		})
		return
	}

	// Cache the result for 24 hours
	if jsonBytes, err := json.Marshal(starterCode); err == nil {
		h.redis.Set(cacheKey, string(jsonBytes), cacheExpiry)
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"starter_code": starterCode,
		"from_cache":   false,
	})
}

// GenerateAllStarterCodes generates and caches starter code for ALL challenges.
// POST /api/ai/generate-all (admin endpoint)
func (h *AIHandler) GenerateAllStarterCodes(w http.ResponseWriter, r *http.Request) {
	challenges, err := h.repo.GetAllChallenges()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch challenges")
		return
	}

	type result struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	results := make([]result, 0, len(challenges))

	for _, ch := range challenges {
		cacheKey := fmt.Sprintf("ai-skel-v2:%s", ch.ID)

		// Skip if already cached
		if cached, err := h.redis.Get(cacheKey); err == nil && cached != "" {
			results = append(results, result{ID: ch.ID, Title: ch.Title, OK: true})
			continue
		}

		var tcBuilder strings.Builder
		for i, tc := range ch.TestCases {
			if i >= 3 {
				break
			}
			tcBuilder.WriteString(fmt.Sprintf("  Input: %s\n  Expected Output: %s\n", tc.Input, tc.ExpectedOutput))
		}

		prompt := fmt.Sprintf(`You are a competitive programming assistant. Generate SKELETON code for ALL 5 languages for this problem.

Problem Title: %s
Problem Description: %s
Test Case Input Format:
%s

For EACH language, generate a SINGLE FILE containing:
1. The SOLUTION FUNCTION with correct signature and parameter types, but with a PLACEHOLDER body only. Do NOT implement the logic.
2. A COMPLETE MAIN FUNCTION that reads stdin, parses each line, calls the function, and prints the result.

Do NOT solve the problem. Skeleton only.
Respond ONLY with raw JSON:
{"python": "...", "cpp": "...", "java": "...", "c": "...", "javascript": "..."}`,
			ch.Title, ch.Description, tcBuilder.String(),
		)

		starterCode, err := h.callGroq(prompt)
		if err != nil {
			results = append(results, result{ID: ch.ID, Title: ch.Title, OK: false, Error: err.Error()})
			continue
		}

		if jsonBytes, merr := json.Marshal(starterCode); merr == nil {
			h.redis.Set(cacheKey, string(jsonBytes), cacheExpiry)
		}

		results = append(results, result{ID: ch.ID, Title: ch.Title, OK: true})

		// Rate limit: be nice to the API
		time.Sleep(500 * time.Millisecond)
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"total":   len(challenges),
	})
}

// callGroq calls the Groq chat completions API and parses the JSON starter code from the response.
func (h *AIHandler) callGroq(prompt string) (map[string]string, error) {
	reqBody := groqRequest{
		Messages: []groqMessage{
			{Role: "system", Content: "You are an expert competitive programming assistant. You ALWAYS respond with only a raw JSON object, never markdown or explanations."},
			{Role: "user", Content: prompt},
		},
		Model:               groqModel,
		Temperature:         0.3, // low temperature for consistent structured output
		MaxCompletionTokens: 4096,
		TopP:                1,
		Stream:              false,
		Stop:                nil,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", groqAPIURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.groqAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("groq API error %d: %s", resp.StatusCode, string(respBody))
	}

	var groqResp groqResponse
	if err := json.Unmarshal(respBody, &groqResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal groq response: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in groq response")
	}

	content := strings.TrimSpace(groqResp.Choices[0].Message.Content)

	// Strip markdown code fences if model wraps in ```json ... ```
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		// Remove first and last fence lines
		if len(lines) >= 3 {
			lines = lines[1 : len(lines)-1]
		}
		content = strings.Join(lines, "\n")
	}

	var starterCode map[string]string
	if err := json.Unmarshal([]byte(content), &starterCode); err != nil {
		return nil, fmt.Errorf("groq returned invalid JSON: %w\nContent: %s", err, content)
	}

	return normalizeStarterCodeMap(starterCode), nil
}

func normalizeStarterCodeMap(input map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range input {
		key := strings.ToLower(strings.TrimSpace(k))
		switch key {
		case "c++", "c_plus_plus":
			key = "cpp"
		case "js", "node":
			key = "javascript"
		case "py":
			key = "python"
		}
		out[key] = v
	}
	return out
}
