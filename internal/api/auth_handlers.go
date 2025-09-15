package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/AgentTarik/finance-api/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"go.uber.org/zap"
)

// TokenIssuer abstracts JWT emission.
type TokenIssuer interface {
	Issue(userID string) (string, time.Time, error)
}

// AuthHandlers handles register/login.
type AuthHandlers struct {
	Log     *zap.Logger
	UsersDB *storage.PostgresStore
	V       *validator.Validate
	Tokens  TokenIssuer
}


// Register godoc
// @Summary      Register a new user
// @Description  Creates a user account.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        payload  body      RegisterRequest  true  "Register payload"
// @Success      201      {object}  map[string]any
// @Failure      400      {object}  map[string]string
// @Failure      409      {object}  map[string]string
// @Router       /auth/register [post]
func (h *AuthHandlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if err := h.V.Struct(req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "validation failed"})
		return
	}

	id, _ := uuid.Parse(req.ID)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	pwHash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	if err := h.UsersDB.CreateUserWithCredentials(c.Request.Context(), id, req.Name, email, string(pwHash)); err != nil {
		// For simplicity, collapse to 409 on uniqueness errors; refine with pq error codes if you want.
		h.Log.Warn("register failed", zap.Error(err))
		c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":    id.String(),
		"name":  req.Name,
		"email": email,
	})
}


// Login godoc
// @Summary      Login with email and password
// @Description  Returns a short-lived JWT access token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        payload  body      LoginRequest  true  "Login payload"
// @Success      200      {object}  map[string]any
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Router       /auth/login [post]
func (h *AuthHandlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if err := h.V.Struct(req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "validation failed"})
		return
	}

	ctx := context.Background()
	email := strings.ToLower(strings.TrimSpace(req.Email))
	u, err := h.UsersDB.GetUserAuthByEmail(ctx, email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, exp, err := h.Tokens.Issue(u.ID.String())
	if err != nil {
		h.Log.Error("jwt issue failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token issue failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   int(exp.Sub(time.Now()).Seconds()),
		"user": gin.H{
			"id":    u.ID.String(),
			"name":  u.Name,
			"email": u.Email,
		},
	})
}
