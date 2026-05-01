package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"codeit/internal/analysis"
	"codeit/internal/auth"
	"codeit/internal/friendbattles"
	"codeit/internal/matches"
	"codeit/internal/matchmaking"
	"codeit/internal/problems"
	"codeit/internal/ratings"
	"codeit/internal/submissions"
	"codeit/internal/users"
	"codeit/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

func Run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL environment variable is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := pgxpool.Connect(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	userRepo := users.NewUserRepository(db)
	userService := users.NewUserService(userRepo)
	userHandler := users.NewUserHandler(userService)
	problemRepo := problems.NewProblemRepository(db)
	problemService := problems.NewProblemService(problemRepo)
	problemHandler := problems.NewProblemHandler(problemService)
	matchRepo := matches.NewMatchRepository(db)
	matchService := matches.NewMatchService(matchRepo)
	matchHandler := matches.NewMatchHandler(matchService)
	matchmakingService := matchmaking.NewService(matchService, problemService)
	wsHub := ws.NewHub()
	matchmakingHandler := matchmaking.NewHandler(matchmakingService, wsHub)
	friendInviteRepo := friendbattles.NewRepository(db)
	friendBattleService := friendbattles.NewService(db, friendInviteRepo, matchService, problemService, userService)
	friendBattleHandler := friendbattles.NewHandler(friendBattleService, wsHub)
	submissionRepo := submissions.NewSubmissionRepository(db)
	judgeClient, err := submissions.NewJudge0Client(
		os.Getenv("JUDGE0_BASE_URL"),
		os.Getenv("JUDGE0_API_KEY"),
		os.Getenv("JUDGE0_RAPIDAPI_HOST"),
	)
	if err != nil {
		return err
	}
	ratingRepo := ratings.NewRepository(db)
	ratingService := ratings.NewService(ratingRepo)
	wsHandler := ws.NewHandler(wsHub, matchService, ratingService)
	submissionService := submissions.NewService(submissionRepo, matchService, problemService, judgeClient, ratingService)
	submissionHandler := submissions.NewHandler(submissionService, wsHub)
	analysisClient := analysis.NewHTTPAnalyzerClient(
		os.Getenv("ANALYZER_API_URL"),
		os.Getenv("ANALYZER_API_KEY"),
		os.Getenv("OPENAI_API_KEY"),
		os.Getenv("OPENAI_MODEL"),
	)
	analysisRepo := analysis.NewRepository(db)
	analysisService := analysis.NewService(matchService, problemService, submissionRepo, analysisRepo, analysisClient)
	analysisHandler := analysis.NewHandler(analysisService)

	router := gin.Default()
	router.Use(corsMiddleware())
	router.Static("/uploads", "./uploads")
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := router.Group("/api/v1")
	{
		api.POST("/auth/register", userHandler.Register)
		api.POST("/auth/login", userHandler.Login)
		api.GET("/users/:id", userHandler.GetPublicProfile)
		api.GET("/users/:id/stats", matchHandler.GetUserMatchStats)
		api.GET("/u/:username", userHandler.GetPublicProfileByUsername)
		api.GET("/leaderboard", userHandler.GetLeaderboard)
		api.GET("/problems", problemHandler.ListProblems)
		api.GET("/problems/random", problemHandler.GetRandomProblemByDifficulty)
		api.GET("/problems/:id", problemHandler.GetProblemByID)
		api.GET("/matches/:id", matchHandler.GetByID)
		api.GET("/friend-battles/:code", friendBattleHandler.GetInvite)

		protected := api.Group("/")
		protected.Use(auth.AuthMiddleware())
		protected.GET("/me", userHandler.GetProfile)
		protected.GET("/me/matches", matchHandler.GetMyMatchHistory)
		protected.GET("/me/stats", matchHandler.GetMyMatchStats)
		protected.PATCH("/me/avatar", userHandler.UpdateMyAvatar)
		protected.POST("/me/avatar/upload", userHandler.UploadMyAvatar)
		protected.PATCH("/users/:id/rating", userHandler.UpdateRating)
		protected.GET("/matches/active", matchHandler.GetActiveMatch)
		protected.POST("/matches/:id/submissions", submissionHandler.Submit)
		protected.POST("/matches/:id/resolve", submissionHandler.ResolveMatch)
		protected.POST("/matches/:id/analyze-last", analysisHandler.AnalyzeLastSubmission)
		protected.GET("/matches/:id/analysis", analysisHandler.GetMatchAnalysis)
		protected.GET("/me/analyses", analysisHandler.ListMyAnalyses)
		protected.POST("/matchmaking", matchmakingHandler.Matchmake)
		protected.DELETE("/matchmaking", matchmakingHandler.LeaveMatchmaking)
		protected.POST("/friend-battles", friendBattleHandler.CreateInvite)
		protected.POST("/friend-battles/:code/join", friendBattleHandler.JoinInvite)
		protected.GET("/ws", wsHandler.HandleWebSocket)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("codeit api listening on :%s", port)
	return router.Run(":" + port)
}

func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:5173"}
	}
	allowedSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowedSet[origin] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowedSet[origin]; ok {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func parseAllowedOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		out = append(out, origin)
	}
	return out
}
