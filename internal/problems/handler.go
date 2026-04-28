package problems

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ProblemHandler struct {
	service *ProblemService
}

func NewProblemHandler(service *ProblemService) *ProblemHandler {
	return &ProblemHandler{service: service}
}

func (h *ProblemHandler) ListProblems(c *gin.Context) {
	problems, err := h.service.ListProblems(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list problems"})
		return
	}
	c.JSON(http.StatusOK, problems)
}

func (h *ProblemHandler) GetProblemByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid problem id"})
		return
	}

	problem, err := h.service.GetProblemByID(c.Request.Context(), id)
	if err != nil {
		if err == ErrInvalidProblemID {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err == ErrProblemNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get problem"})
		return
	}

	c.JSON(http.StatusOK, problem)
}

func (h *ProblemHandler) GetRandomProblemByDifficulty(c *gin.Context) {
	difficulty := c.Query("difficulty")
	problem, err := h.service.GetRandomProblemByDifficulty(c.Request.Context(), difficulty)
	if err != nil {
		if err == ErrInvalidDifficulty {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err == ErrProblemNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get random problem"})
		return
	}

	c.JSON(http.StatusOK, problem)
}
