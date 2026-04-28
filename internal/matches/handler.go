package matches

import (
	"net/http"
	"strconv"
	"strings"

	"codeit/internal/auth"
	"github.com/gin-gonic/gin"
)

type MatchHandler struct {
	service *MatchService
}

func NewMatchHandler(service *MatchService) *MatchHandler {
	return &MatchHandler{service: service}
}

func (h *MatchHandler) GetByID(c *gin.Context) {
	matchID := c.Param("id")
	match, err := h.service.GetByID(c.Request.Context(), matchID)
	if err != nil {
		if err == ErrInvalidMatchID {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err == ErrMatchNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get match"})
		return
	}

	c.JSON(http.StatusOK, match)
}

func (h *MatchHandler) GetActiveMatch(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	match, err := h.service.GetActiveMatchByUserID(c.Request.Context(), userID)
	if err != nil {
		if err == ErrInvalidUserID {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err == ErrMatchNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get active match"})
		return
	}

	c.JSON(http.StatusOK, match)
}

func (h *MatchHandler) GetMyMatchHistory(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit := parseHistoryInt(c.DefaultQuery("limit", ""), defaultHistoryLimit)
	offset := parseHistoryInt(c.DefaultQuery("offset", ""), 0)
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}
	if offset < 0 {
		offset = 0
	}

	page, err := h.service.ListFinishedMatchHistory(c.Request.Context(), userID, limit, offset)
	if err != nil {
		if err == ErrInvalidUserID {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load match history"})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *MatchHandler) GetMyMatchStats(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	stats, err := h.service.GetUserMatchStats(c.Request.Context(), userID)
	if err != nil {
		if err == ErrInvalidUserID {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *MatchHandler) GetUserMatchStats(c *gin.Context) {
	userID := strings.TrimSpace(c.Param("id"))
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrInvalidUserID.Error()})
		return
	}

	stats, err := h.service.GetUserMatchStats(c.Request.Context(), userID)
	if err != nil {
		if err == ErrInvalidUserID {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func parseHistoryInt(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
