package handlers

import (
	"bytes"
	"code-mafia-backend/internal/config"
	"code-mafia-backend/internal/database"
	"code-mafia-backend/internal/middleware"
	"code-mafia-backend/internal/models"
	"code-mafia-backend/internal/redis"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type EditorHandler struct {
	repo   *database.Repository
	redis  *redis.Client
	config *config.Config
}

func NewEditorHandler(repo *database.Repository, redisClient *redis.Client, cfg *config.Config) *EditorHandler {
	return &EditorHandler{
		repo:   repo,
		redis:  redisClient,
		config: cfg,
	}
}

type CodeSubmissionRequest struct {
	QuestionID string `json:"question_id"`
	LanguageID int    `json:"language_id"`
	SourceCode string `json:"source_code"`
}

// rawCodeRequest accepts question_id as string or number (JSON from frontend).
type rawCodeRequest struct {
	QuestionID interface{} `json:"question_id"`
	LanguageID int         `json:"language_id"`
	SourceCode string      `json:"source_code"`
}

func normalizeQuestionID(v interface{}) (string, error) {
	if v == nil {
		return "", fmt.Errorf("missing question_id")
	}
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return "", fmt.Errorf("invalid question_id")
		}
		return x, nil
	case float64:
		return strconv.FormatInt(int64(x), 10), nil
	case json.Number:
		return x.String(), nil
	default:
		return fmt.Sprint(x), nil
	}
}

func (h *EditorHandler) RunTestCases(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var raw rawCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	qid, err := normalizeQuestionID(raw.QuestionID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	req := CodeSubmissionRequest{QuestionID: qid, LanguageID: raw.LanguageID, SourceCode: raw.SourceCode}

	// Get challenge from cache or database
	challenge, err := h.getChallenge(req.QuestionID, false)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Challenge not found")
		return
	}

	// Extract test cases
	testCases, err := h.extractTestCases(challenge, false)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error extracting test cases")
		return
	}

	results, summary, err := h.runJudge0Pipeline(testCases, req.LanguageID, req.SourceCode)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Judge0 error: %v", err))
		return
	}

	passed := 0
	for _, result := range results {
		if result.IsCorrect {
			passed++
		}
	}

	response := models.SubmissionResponse{
		ChallengeID: challenge.ID,
		Title:       challenge.Title,
		Results:     results,
		Passed:      passed,
		Total:       len(testCases),
		Summary:     summary,
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (h *EditorHandler) SubmitQuestion(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var raw rawCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	qid, err := normalizeQuestionID(raw.QuestionID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	req := CodeSubmissionRequest{QuestionID: qid, LanguageID: raw.LanguageID, SourceCode: raw.SourceCode}

	// Get challenge with all test cases (including hidden)
	challenge, err := h.getChallenge(req.QuestionID, true)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Challenge not found")
		return
	}

	// Extract all test cases
	testCases, err := h.extractTestCases(challenge, true)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error extracting test cases")
		return
	}

	results, summary, err := h.runJudge0Pipeline(testCases, req.LanguageID, req.SourceCode)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Judge0 error: %v", err))
		return
	}

	passed := 0
	for _, result := range results {
		if result.IsCorrect {
			passed++
		}
	}

	// Store submission
	if err := h.storeSubmission(user.TeamID, challenge.ID, req.SourceCode, passed, len(testCases), challenge.Points); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error storing submission")
		return
	}

	response := models.SubmissionResponse{
		ChallengeID: challenge.ID,
		Title:       challenge.Title,
		Results:     results,
		Passed:      passed,
		Total:       len(testCases),
		Summary:     summary,
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (h *EditorHandler) GetPoints(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	team, err := h.repo.GetTeamByID(user.TeamID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching points")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]int{"points": team.Points})
}

func (h *EditorHandler) getChallenge(id string, includeHidden bool) (*models.Challenge, error) {
	// Try to get from cache first
	cacheKey := "challenges-user"
	if includeHidden {
		cacheKey = "challenges-judge0"
	}

	cachedData, err := h.redis.Get(cacheKey)
	if err == nil && cachedData != "" {
		var challenges []models.Challenge
		if err := json.Unmarshal([]byte(cachedData), &challenges); err == nil {
			for _, challenge := range challenges {
				if challenge.ID == id {
					return &challenge, nil
				}
			}
		}
	}

	// Fallback to database
	challenge, err := h.repo.GetChallengeByID(id)
	if err != nil {
		return nil, err
	}
	if challenge == nil {
		return nil, fmt.Errorf("challenge not found")
	}
	return challenge, nil
}

func (h *EditorHandler) extractTestCases(challenge *models.Challenge, includeHidden bool) ([]models.TestCase, error) {
	var testCases []models.TestCase
	for _, tc := range challenge.TestCases {
		if !includeHidden && tc.Type == "hidden" {
			continue
		}
		testCases = append(testCases, tc)
	}

	return testCases, nil
}

func (h *EditorHandler) runJudge0Pipeline(testCases []models.TestCase, languageID int, sourceCode string) ([]models.TestResult, string, error) {
	results, err := h.submitToJudge0(testCases, languageID, sourceCode)
	if err != nil {
		return nil, "", err
	}
	return results, buildSummary(results), nil
}

func buildSummary(results []models.TestResult) string {
	if len(results) == 0 {
		return "Error"
	}
	passed := 0
	for _, r := range results {
		if r.IsCorrect {
			passed++
		}
	}
	if passed == len(results) {
		return "Accepted"
	}
	if passed > 0 {
		return "Partial"
	}
	var first *models.TestResult
	for i := range results {
		if !results[i].IsCorrect {
			first = &results[i]
			break
		}
	}
	if first == nil {
		return "Error"
	}
	switch first.StatusID {
	case 6:
		return "Compilation Error"
	case 5:
		return "Time Limit Exceeded"
	case 4:
		return "Wrong Answer"
	default:
		return first.Status
	}
}

func wrapSourceCode(languageID int, sourceCode string) string {
	// If code already has a main/entry point (AI-generated full skeleton), don't wrap
	if strings.Contains(sourceCode, "if __name__") ||
		strings.Contains(sourceCode, "process.stdin") ||
		strings.Contains(sourceCode, "int main(") ||
		strings.Contains(sourceCode, "public static void main") {
		return sourceCode
	}

	if languageID == 71 && !strings.Contains(sourceCode, "print(") {
		wrapper := `
import sys, ast, json, inspect

%s

def __run__():
    content = sys.stdin.read().strip()
    if not content: return
    lines = content.splitlines()
    
    funcs = [obj for name, obj in list(globals().items()) if inspect.isfunction(obj) and obj.__module__ == '__main__' and name != '__run__']
    if not funcs: return
    func = funcs[-1]
    
    args = []
    for line in lines:
        if not line.strip(): continue
        try:
            args.append(ast.literal_eval(line))
        except:
            try:
                args.append(json.loads(line))
            except:
                args.append(line)
                
    try:
        res = func(*args)
        if res is not None:
            if isinstance(res, (list, dict)):
                print(json.dumps(res, separators=(',', ':')))
            elif isinstance(res, bool):
                print(str(res).lower())
            else:
                print(res)
    except Exception as e:
        print("Runtime Error:", e)

if __name__ == '__main__':
    __run__()
`
		return fmt.Sprintf(wrapper, sourceCode)
	} else if languageID == 63 && !strings.Contains(sourceCode, "console.log(") {
		// JavaScript Dynamic Execution (Language 63)
		re := regexp.MustCompile(`function\s+([a-zA-Z0-9_]+)\s*\(|const\s+([a-zA-Z0-9_]+)\s*=\s*(?:\([^)]*\)|[a-zA-Z0-9_]+)\s*=>|let\s+([a-zA-Z0-9_]+)\s*=\s*(?:\([^)]*\)|[a-zA-Z0-9_]+)\s*=>`)
		matches := re.FindStringSubmatch(sourceCode)
		var funcName string
		if len(matches) > 0 {
			for i := 1; i < len(matches); i++ {
				if matches[i] != "" {
					funcName = matches[i]
					break
				}
			}
		}

		if funcName != "" {
			wrapper := `
const fs = require('fs');

%s

function __run__() {
    let content = "";
    try {
        content = fs.readFileSync(0, 'utf-8').trim();
    } catch (e) {
        return;
    }
    if (!content) return;
    
    const lines = content.split('\n');
    const args = [];
    for (const line of lines) {
        if (!line.trim()) continue;
        try { args.push(JSON.parse(line)); } catch(e) { args.push(line); }
    }
    
    try {
        const res = %s(...args);
        if (res !== undefined) {
            if (typeof res === 'object') console.log(JSON.stringify(res).replace(/\s/g, ''));
            else console.log(res);
        }
    } catch(err) {
        console.error("Runtime Error:", err);
    }
}
__run__();
`
			return fmt.Sprintf(wrapper, sourceCode, funcName)
		}
	}
	return sourceCode
}

func (h *EditorHandler) submitToJudge0(testCases []models.TestCase, languageID int, sourceCode string) ([]models.TestResult, error) {
	// Prepare submissions
	var submissions []models.Judge0Submission
	finalSource := wrapSourceCode(languageID, sourceCode)

	for _, tc := range testCases {
		submissions = append(submissions, models.Judge0Submission{
			SourceCode:     base64.StdEncoding.EncodeToString([]byte(finalSource)),
			LanguageID:     languageID,
			Stdin:          base64.StdEncoding.EncodeToString([]byte(tc.Input)),
			ExpectedOutput: base64.StdEncoding.EncodeToString([]byte(tc.ExpectedOutput)),
		})
	}

	// Submit batch
	batchRequest := map[string]interface{}{
		"submissions": submissions,
	}
	jsonData, _ := json.Marshal(batchRequest)

	api := strings.TrimSuffix(h.config.RapidAPIURL, "/")
	req, err := http.NewRequest("POST", api+"/submissions/batch?base64_encoded=true", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if h.config.RapidAPIKey != "" {
		req.Header.Set("x-rapidapi-key", h.config.RapidAPIKey)
		req.Header.Set("x-rapidapi-host", h.config.RapidAPIHost)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("judge0 POST error: %d - %s", resp.StatusCode, string(body))
	}
	
	var batchResponse []models.Judge0Response
	if err := json.Unmarshal(body, &batchResponse); err != nil {
		fmt.Printf("Error unmarshaling POST batchResponse: %v, body: %s\n", err, string(body))
		return nil, err
	}

	// Get tokens
	var tokens []string
	for _, r := range batchResponse {
		tokens = append(tokens, r.Token)
	}
	tokenString := strings.Join(tokens, ",")

	// Poll for results
	return h.pollJudge0(tokenString, testCases)
}

func (h *EditorHandler) pollJudge0(tokens string, testCases []models.TestCase) ([]models.TestResult, error) {
	api := strings.TrimSuffix(h.config.RapidAPIURL, "/")
	parts := strings.Split(tokens, ",")
	escaped := make([]string, len(parts))
	for i, t := range parts {
		escaped[i] = url.QueryEscape(strings.TrimSpace(t))
	}
	tokensQS := strings.Join(escaped, ",")

	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2000 * time.Millisecond) // RapidAPI is 1 req / sec
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("judge0 request timeout")
		case <-ticker.C:
			u := fmt.Sprintf("%s/submissions/batch?tokens=%s&fields=*&base64_encoded=true", api, tokensQS)
			req, _ := http.NewRequest("GET", u, nil)
			if h.config.RapidAPIKey != "" {
				req.Header.Set("x-rapidapi-key", h.config.RapidAPIKey)
				req.Header.Set("x-rapidapi-host", h.config.RapidAPIHost)
			}

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("judge0 GET error: %d - %s\n", resp.StatusCode, string(body))
				// Just continue and try again later, they might be rate limiting us.
				continue 
			}

			var batchResponse models.Judge0BatchResponse
			if err := json.Unmarshal(body, &batchResponse); err != nil {
				fmt.Printf("Error unmarshaling GET batchResponse: %v, body: %s\n", err, string(body))
				continue
			}

			// Check if all completed
			allCompleted := true
			for _, sub := range batchResponse.Submissions {
				if sub.Status.ID <= 2 {
					allCompleted = false
					break
				}
			}

			if allCompleted {
				return h.decodeResults(batchResponse, testCases), nil
			}
		}
	}
}

func judge0DecodeB64(s *string) string {
	if s == nil || *s == "" {
		return ""
	}
	b, err := base64.StdEncoding.DecodeString(*s)
	if err != nil {
		return ""
	}
	return string(b)
}

func normalizeForCompare(s string) string {
	normalized := strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "\n", "")
	normalized = strings.ReplaceAll(normalized, "\r", "")

	// Handle cases like expected "\"olleh\"" vs actual "olleh".
	if unquoted, err := strconv.Unquote(normalized); err == nil {
		normalized = unquoted
		normalized = strings.ReplaceAll(strings.ReplaceAll(normalized, " ", ""), "\n", "")
		normalized = strings.ReplaceAll(normalized, "\r", "")
	}

	lower := strings.ToLower(normalized)
	if lower == "true" || lower == "false" {
		return lower
	}
	return normalized
}

func (h *EditorHandler) decodeResults(response models.Judge0BatchResponse, testCases []models.TestCase) []models.TestResult {
	var results []models.TestResult
	for i, sub := range response.Submissions {
		tc := testCases[i]

		output := judge0DecodeB64(sub.Stdout)
		stderr := judge0DecodeB64(sub.Stderr)
		compileOut := judge0DecodeB64(sub.CompileOutput)
		var msg string
		if sub.Message != nil {
			msg = *sub.Message
		}

		isHidden := tc.Type == "hidden"
		expOut := tc.ExpectedOutput

		// Lenient Output Evaluation:
		// - ignores whitespace/newline differences
		// - normalizes quoted strings
		// - compares booleans case-insensitively (True/False vs true/false)
		cleanActual := normalizeForCompare(output)
		cleanExpected := normalizeForCompare(expOut)

		isCorrect := (cleanActual == cleanExpected) && (sub.Status.ID == 3 || sub.Status.ID == 4)
		
		statusID := sub.Status.ID
		statusDesc := sub.Status.Description
		if isCorrect {
			statusID = 3
			statusDesc = "Accepted"
		} else if statusID == 3 {
			// If Judge0 marked accepted but we consider wrong (rare), or if we just want to enforce our strictness
			// Basically if it's Not Correct manually, but Status ID is 3, something mismatches deeply
			statusID = 4
			statusDesc = "Wrong Answer"
		}

		runtime := "N/A"
		if sub.Time != nil {
			runtime = fmt.Sprintf("%ss", *sub.Time)
		}

		result := models.TestResult{
			TestCase:       tc.Name,
			StatusID:       statusID,
			Status:         statusDesc,
			Input:          tc.Input,
			Output:         output,
			ExpectedOutput: expOut,
			Stderr:         stderr,
			CompileOutput:  compileOut,
			Message:        msg,
			IsCorrect:      isCorrect,
			Runtime:        runtime,
		}

		if isHidden {
			result.Input = "hidden"
			result.Output = "hidden"
			result.ExpectedOutput = "hidden"
			result.Stderr = "hidden"
		}

		results = append(results, result)
	}
	return results
}

func (h *EditorHandler) storeSubmission(teamID string, challengeID, sourceCode string, passed, total, challengePoints int) error {
	// Determine status
	status := "Incomplete"
	if passed == total {
		status = "Accepted"
	} else if passed > 0 {
		status = "Partial"
	}

	// Calculate coins
	coins := 0
	if challengePoints == 10 {
		coins = 5
	} else if challengePoints == 20 {
		coins = 7
	} else if challengePoints == 30 {
		coins = 10
	}

	// Calculate points awarded
	pointsAwarded := (passed * challengePoints) / total

	// Check if submission exists
	existing, err := h.repo.GetSubmissionByTeamAndChallenge(teamID, challengeID)
	if err != nil {
		return err
	}

	if existing == nil {
		// Create new submission
		if err := h.repo.CreateSubmission(teamID, challengeID, sourceCode, status, pointsAwarded); err != nil {
			return err
		}

		// Update team points and coins
		team, err := h.repo.GetTeamByID(teamID)
		if err != nil {
			return err
		}

		newPoints := team.Points + pointsAwarded
		newCoins := team.Coins
		if passed == total {
			newCoins += coins
		}

		if err := h.repo.UpdateTeamPointsAndCoins(teamID, newPoints, newCoins); err != nil {
			return err
		}
		h.refreshLeaderboardCache()
		return nil
	} else if pointsAwarded >= existing.PointsAwarded {
		// Update existing submission
		scoreDiff := pointsAwarded - existing.PointsAwarded

		if err := h.repo.UpdateSubmission(existing.ID, sourceCode, status, pointsAwarded); err != nil {
			return err
		}

		// Update team points
		team, err := h.repo.GetTeamByID(teamID)
		if err != nil {
			return err
		}

		newPoints := team.Points + scoreDiff
		newCoins := team.Coins
		if passed == total && existing.PointsAwarded < challengePoints {
			newCoins += coins
		}

		if err := h.repo.UpdateTeamPointsAndCoins(teamID, newPoints, newCoins); err != nil {
			return err
		}
		h.refreshLeaderboardCache()
		return nil
	}

	return nil
}

func (h *EditorHandler) refreshLeaderboardCache() {
	teams, err := h.repo.GetAllTeams()
	if err != nil {
		return
	}
	data, err := json.Marshal(teams)
	if err != nil {
		return
	}
	_ = h.redis.SetLeaderboard(string(data))
}
