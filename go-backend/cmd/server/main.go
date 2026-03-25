package main

import (
	"code-mafia-backend/internal/config"
	"code-mafia-backend/internal/database"
	"code-mafia-backend/internal/handlers"
	"code-mafia-backend/internal/middleware"
	"code-mafia-backend/internal/redis"
	"code-mafia-backend/internal/websocket"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize MongoDB
	_, err := database.Connect(cfg.MongoDBURI, cfg.MongoDBDatabase)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer database.Disconnect()

	// Initialize Redis
	redisClient := redis.NewClient(cfg.RedisAddress, cfg.RedisPassword)
	defer redisClient.Close()

	// Initialize repository
	repo := database.NewRepository(database.DB)

	// Initialize WebSocket hub
	hub := websocket.NewHub(redisClient, repo, cfg.SecretKey)
	go hub.Run()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(repo, redisClient, cfg.SecretKey)
	editorHandler := handlers.NewEditorHandler(repo, redisClient, cfg)
	gameHandler := handlers.NewGameHandler(repo, redisClient)
	problemHandler := handlers.NewProblemHandler(repo, redisClient)
	leaderHandler := handlers.NewLeaderHandler(repo, redisClient)
	adminHandler := handlers.NewAdminHandler(repo, redisClient, cfg.SecretKey)
	aiHandler := handlers.NewAIHandler(repo, redisClient, cfg.GroqAPIKey)

	// Setup router
	router := mux.NewRouter()

	// Health check
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<h1>Code Mafia API ENDPOINT</h1>"))
	}).Methods("GET")

	// Auth routes
	authRouter := router.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/login", authHandler.Login).Methods("POST")
	authRouter.Handle("/signup", middleware.AdminVerify(cfg.SecretKey)(http.HandlerFunc(adminHandler.Signup))).Methods("POST")
	authRouter.Handle("/verify", middleware.VerifyToken(cfg.SecretKey, redisClient)(http.HandlerFunc(authHandler.Verify))).Methods("GET")
	authRouter.Handle("/logout", middleware.VerifyToken(cfg.SecretKey, redisClient)(http.HandlerFunc(authHandler.Logout))).Methods("POST")

	// API routes
	apiRouter := router.PathPrefix("/api").Subrouter()

	// Editor routes
	editorRouter := apiRouter.PathPrefix("/editor").Subrouter()
	editorRouter.Use(middleware.VerifyToken(cfg.SecretKey, redisClient))
	editorRouter.HandleFunc("/runtestcases", editorHandler.RunTestCases).Methods("POST")
	editorRouter.HandleFunc("/submitquestion", editorHandler.SubmitQuestion).Methods("POST")
	editorRouter.HandleFunc("/points", editorHandler.GetPoints).Methods("GET")

	// Game routes
	gameRouter := apiRouter.PathPrefix("/game").Subrouter()
	gameRouter.HandleFunc("/getpowers", gameHandler.GetPowers).Methods("GET")
	gameRouter.Handle("/getcoins", middleware.VerifyToken(cfg.SecretKey, redisClient)(http.HandlerFunc(gameHandler.GetCoins))).Methods("GET")
	gameRouter.HandleFunc("/status", adminHandler.GetGameStatus).Methods("GET") // Public endpoint for game status

	// Problem routes
	problemRouter := apiRouter.PathPrefix("/problem").Subrouter()
	problemRouter.HandleFunc("", problemHandler.GetProblems).Methods("GET")
	problemRouter.Handle("/status", middleware.VerifyToken(cfg.SecretKey, redisClient)(http.HandlerFunc(problemHandler.GetChallengesSolvedStatus))).Methods("GET")

	// Leader routes
	apiRouter.HandleFunc("/leader", leaderHandler.GetLeaderboard).Methods("GET")

	// Admin routes
	adminRouter := apiRouter.PathPrefix("/admin").Subrouter()
	adminRouter.Use(middleware.AdminVerify(cfg.SecretKey))
	adminRouter.HandleFunc("/problems/refresh-cache", adminHandler.RefreshCache).Methods("GET")
	adminRouter.HandleFunc("/problems/upload", adminHandler.UploadProblem).Methods("POST")
	adminRouter.HandleFunc("/problems/{id}/testcases", adminHandler.UploadTestCases).Methods("POST")
	adminRouter.HandleFunc("/problems/{id}", adminHandler.DeleteChallenge).Methods("DELETE")
	adminRouter.HandleFunc("/problems", adminHandler.GetAllChallenges).Methods("GET")
	adminRouter.HandleFunc("/game/status", adminHandler.GetGameStatus).Methods("GET")
	adminRouter.HandleFunc("/game/control", adminHandler.ControlGame).Methods("POST")
	adminRouter.HandleFunc("/teams", adminHandler.GetAllTeams).Methods("GET")
	adminRouter.HandleFunc("/teams/create", adminHandler.CreateTeam).Methods("POST")
	adminRouter.HandleFunc("/ai/generate-all", aiHandler.GenerateAllStarterCodes).Methods("POST")

	// AI routes (public — results are cached, no sensitive data)
	aiRouter := apiRouter.PathPrefix("/ai").Subrouter()
	aiRouter.HandleFunc("/starter", aiHandler.GenerateStarterCode).Methods("GET")

	// WebSocket route
	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWs(hub, w, r, cfg.SecretKey)
	})

	// CORS configuration
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
	})

	handler := c.Handler(router)

	// Start leaderboard update goroutine
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			leaderHandler.UpdateLeaderboard()
		}
	}()

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Server starting on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
