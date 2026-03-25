package handlers

import (
	"code-mafia-backend/internal/database"
	"code-mafia-backend/internal/models"
	"code-mafia-backend/internal/redis"
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

type AdminHandler struct {
	repo      *database.Repository
	redis     *redis.Client
	secretKey string
}

func NewAdminHandler(repo *database.Repository, redisClient *redis.Client, secretKey string) *AdminHandler {
	return &AdminHandler{
		repo:      repo,
		redis:     redisClient,
		secretKey: secretKey,
	}
}

type SignupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TeamName string `json:"team_name"`
}

type ProblemUploadRequest struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Difficulty  string            `json:"difficulty"`
	Points      int               `json:"points"`
	TestCases   []models.TestCase `json:"test_cases"`
	StarterCode map[string]string `json:"starter_code"`
}

type GameControlRequest struct {
	Action string `json:"action"` // "start", "pause", "stop", "reset"
}

type CreateTeamRequest struct {
	TeamName string `json:"team_name"`
}

func (h *AdminHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Check if user exists
	existingUser, err := h.repo.GetUserByUsername(req.Username)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if existingUser != nil {
		respondWithError(w, http.StatusConflict, "Username already exists")
		return
	}

	// Get or create team
	var teamID string
	team, err := h.repo.GetTeamByName(req.TeamName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if team != nil {
		teamID = team.ID
	} else {
		newTeam, err := h.repo.CreateTeam(req.TeamName)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to create team")
			return
		}
		teamID = newTeam.ID
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Create user
	user, err := h.repo.CreateUser(req.Username, string(hashedPassword), teamID, "user")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Get team info for response
	team, _ = h.repo.GetTeamByID(teamID)

	// Create JWT token
	claims := jwt.MapClaims{
		"username":  user.Username,
		"team_id":   user.TeamID,
		"team_name": team.Name,
		"role":      user.Role,
		"exp":       time.Now().Add(6 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.secretKey))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Store token in Redis
	h.redis.SetToken(user.Username, tokenString, 6*time.Hour)

	respondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "User created successfully",
		"token":   tokenString,
		"user": map[string]interface{}{
			"username":  user.Username,
			"team_name": team.Name,
			"role":      user.Role,
		},
	})
}

func (h *AdminHandler) RefreshCache(w http.ResponseWriter, r *http.Request) {
	if err := h.refreshChallengeCaches(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching challenges")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Challenge cache refreshed successfully"})
}

func (h *AdminHandler) refreshChallengeCaches() error {
	// Fetch all challenges from database
	challenges, err := h.repo.GetAllChallenges()
	if err != nil {
		return err
	}

	// Cache for user (without hidden test cases)
	userChallenges := make([]models.Challenge, len(challenges))
	copy(userChallenges, challenges)

	for i := range userChallenges {
		filteredTestCases := []models.TestCase{}
		for _, tc := range userChallenges[i].TestCases {
			if tc.Type != "hidden" {
				filteredTestCases = append(filteredTestCases, tc)
			}
		}
		userChallenges[i].TestCases = filteredTestCases
	}

	userCacheData, _ := json.Marshal(userChallenges)
	h.redis.Set("challenges-user", string(userCacheData), 0)

	// Cache for judge0 (with all test cases)
	judge0CacheData, _ := json.Marshal(challenges)
	h.redis.Set("challenges-judge0", string(judge0CacheData), 0)

	return nil
}

func (h *AdminHandler) UploadProblem(w http.ResponseWriter, r *http.Request) {
	var req ProblemUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.ID == "" || req.Title == "" || req.Description == "" {
		respondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	// Create challenge
	if err := h.repo.CreateChallenge(req.ID, req.Title, req.Description, req.Difficulty, req.Points, req.TestCases, req.StarterCode); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create challenge")
		return
	}

	// Refresh cache
	if err := h.refreshChallengeCaches(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Problem created, but failed to refresh challenge cache")
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{"message": "Problem uploaded successfully"})
}

func (h *AdminHandler) UploadTestCases(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	challengeID := vars["id"]

	var testCases []models.TestCase
	if err := json.NewDecoder(r.Body).Decode(&testCases); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get existing challenge
	challenge, err := h.repo.GetChallengeByID(challengeID)
	if err != nil || challenge == nil {
		respondWithError(w, http.StatusNotFound, "Challenge not found")
		return
	}

	// Update test cases
	if err := h.repo.CreateChallenge(challenge.ID, challenge.Title, challenge.Description, challenge.Difficulty, challenge.Points, testCases, challenge.StarterCode); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update test cases")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Test cases updated successfully"})
}

func (h *AdminHandler) DeleteChallenge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	challengeID := vars["id"]
	if challengeID == "" {
		respondWithError(w, http.StatusBadRequest, "Challenge id is required")
		return
	}

	challenge, err := h.repo.GetChallengeByID(challengeID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch challenge")
		return
	}
	if challenge == nil {
		respondWithError(w, http.StatusNotFound, "Challenge not found")
		return
	}

	if err := h.repo.DeleteChallengeByID(challengeID); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete challenge")
		return
	}

	h.redis.Delete("ai-skel-v2:" + challengeID)
	h.redis.Delete("ai-skel-v2:" + challenge.ID)

	if err := h.refreshChallengeCaches(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Challenge deleted, but failed to refresh challenge cache")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Challenge deleted successfully"})
}

func (h *AdminHandler) GetGameStatus(w http.ResponseWriter, r *http.Request) {
	// Get game status from Redis
	status, err := h.redis.Get("game:status")
	if err != nil {
		status = "stopped"
	}

	// Get statistics
	teams, _ := h.repo.GetAllTeams()

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":     status,
		"totalTeams": len(teams),
	})
}

func (h *AdminHandler) ControlGame(w http.ResponseWriter, r *http.Request) {
	var req GameControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate action
	validActions := map[string]bool{"start": true, "pause": true, "stop": true, "reset": true}
	if !validActions[req.Action] {
		respondWithError(w, http.StatusBadRequest, "Invalid action")
		return
	}

	// If starting the game, check if problems exist
	if req.Action == "start" {
		challenges, err := h.repo.GetAllChallenges()
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to check problems")
			return
		}
		if len(challenges) == 0 {
			respondWithError(w, http.StatusBadRequest, "Cannot start game: No problems uploaded. Please upload at least one problem first.")
			return
		}
	}

	// Update game status
	h.redis.Set("game:status", req.Action, 0)

	// Broadcast game status change via WebSocket
	h.redis.Publish("game:control", req.Action)

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Game status updated to " + req.Action,
		"status":  req.Action,
	})
}

func (h *AdminHandler) GetAllTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := h.repo.GetAllTeams()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch teams")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"teams": teams,
		"count": len(teams),
	})
}

func (h *AdminHandler) GetAllChallenges(w http.ResponseWriter, r *http.Request) {
	challenges, err := h.repo.GetAllChallenges()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch challenges")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"challenges": challenges,
		"count":      len(challenges),
	})
}

func (h *AdminHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.TeamName == "" {
		respondWithError(w, http.StatusBadRequest, "Team name is required")
		return
	}

	// Check if team already exists
	existingTeam, err := h.repo.GetTeamByName(req.TeamName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if existingTeam != nil {
		respondWithError(w, http.StatusConflict, "Team already exists")
		return
	}

	// Create team
	team, err := h.repo.CreateTeam(req.TeamName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create team")
		return
	}

	// Create a default user for this team
	// Username will be the team name (lowercase, no spaces)
	// Default password will be "team123"
	username := req.TeamName
	defaultPassword := "team123"

	// Check if user already exists
	existingUser, err := h.repo.GetUserByUsername(username)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error checking user")
		return
	}

	var userCreated bool
	if existingUser == nil {
		// Hash the default password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error hashing password")
			return
		}

		// Create user with the correct signature
		_, err = h.repo.CreateUser(username, string(hashedPassword), team.ID, "user")
		if err != nil {
			// If user creation fails, we should still return the team
			// but indicate that user creation failed
			respondWithJSON(w, http.StatusCreated, map[string]interface{}{
				"message":      "Team created but user creation failed",
				"team":         team,
				"user_created": false,
				"error":        "Failed to create user",
			})
			return
		}
		userCreated = true
	}

	respondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"message":      "Team created successfully",
		"team":         team,
		"user_created": userCreated,
		"username":     username,
		"password":     defaultPassword,
	})
}
