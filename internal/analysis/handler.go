package analysis

import (
	"net/http"
	"strconv"
	"strings"

	"codeit/internal/auth"
	"codeit/internal/matches"
	"codeit/internal/problems"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) AnalyzeLastSubmission(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	refresh := parseBoolQuery(c.Query("refresh"))
	res, err := h.service.AnalyzeLastSubmission(c.Request.Context(), matchID, userID, refresh)
	if err != nil {
		switch err {
		case ErrInvalidInput:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case ErrUnauthorizedForMatch:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case ErrNoSubmissionsYet:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case matches.ErrMatchNotFound, problems.ErrProblemNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrAnalyzerUnavailable:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to analyze submission"})
		}
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) GetMatchAnalysis(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	matchID := c.Param("id")
	res, err := h.service.GetLatestMatchAnalysis(c.Request.Context(), matchID, userID)
	if err != nil {
		switch err {
		case ErrInvalidInput:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case ErrUnauthorizedForMatch:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case ErrNoSubmissionsYet:
			c.JSON(http.StatusNotFound, gin.H{"error": "analysis not found"})
		case matches.ErrMatchNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get match analysis"})
		}
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ListMyAnalyses(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	limit := parseInt(c.Query("limit"), 20)
	offset := parseInt(c.Query("offset"), 0)
	res, err := h.service.ListMyAnalyses(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load analyses"})
		return
	}
	c.JSON(http.StatusOK, res)
}

func parseBoolQuery(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "1" || v == "true" || v == "yes"
}

func parseInt(v string, d int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return d
	}
	return n
}
