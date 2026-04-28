package users

import (
	"codeit/internal/auth"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	service *UserService
}

func NewUserHandler(service *UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	user, err := h.service.Register(c.Request.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		if err == ErrInvalidInput {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register user"})
		}
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (h *UserHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	user, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if err == ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
		}
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.service.GetPublicProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get profile"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) GetPublicProfile(c *gin.Context) {
	userID := c.Param("id")
	user, err := h.service.GetPublicProfile(c.Request.Context(), userID)
	if err != nil {
		switch err {
		case ErrInvalidInput:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get public profile"})
		}
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) GetPublicProfileByUsername(c *gin.Context) {
	username := c.Param("username")
	user, err := h.service.GetPublicProfileByUsername(c.Request.Context(), username)
	if err != nil {
		switch err {
		case ErrInvalidInput:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case ErrUserNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get public profile"})
		}
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) GetLeaderboard(c *gin.Context) {
	limit := 50
	offset := 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	entries, err := h.service.GetLeaderboard(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load leaderboard"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

func (h *UserHandler) UpdateRating(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		Rating int `json:"rating"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.service.UpdateRating(c.Request.Context(), userID, req.Rating); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update rating"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "rating updated successfully"})
}

func (h *UserHandler) UpdateMyAvatar(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.service.UpdateAvatar(c.Request.Context(), userID, req.AvatarURL); err != nil {
		if err == ErrInvalidInput {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update avatar"})
		return
	}
	user, err := h.service.GetPublicProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "avatar updated but failed to load profile"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) UploadMyAvatar(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "avatar file is required"})
		return
	}

	const maxAvatarBytes = 5 * 1024 * 1024 // 5 MB
	if file.Size > maxAvatarBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "avatar file is too large (max 5MB)"})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported avatar format"})
		return
	}

	dir := filepath.Join("uploads", "avatars")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to prepare avatar storage"})
		return
	}

	filename := uuid.New().String() + ext
	dst := filepath.Join(dir, filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save avatar"})
		return
	}

	avatarPath := "/uploads/avatars/" + filename
	if err := h.service.UpdateAvatar(c.Request.Context(), userID, avatarPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update avatar"})
		return
	}

	user, err := h.service.GetPublicProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "avatar uploaded but failed to load profile"})
		return
	}
	c.JSON(http.StatusOK, user)
}
