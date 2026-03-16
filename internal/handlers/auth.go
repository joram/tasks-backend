package handlers

import (
	"crypto/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"task-tracker-api/internal/config"
	"task-tracker-api/internal/models"
)

const saltSize = 16

type AuthHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

// hashPasswordWithSalt returns bcrypt(salt + password). Salt may be nil for legacy (no salt).
func hashPasswordWithSalt(salt []byte, password string) ([]byte, error) {
	input := []byte(password)
	if len(salt) > 0 {
		input = append(salt, input...)
	}
	return bcrypt.GenerateFromPassword(input, bcrypt.DefaultCost)
}

func verifyPassword(hash string, salt []byte, password string) bool {
	input := []byte(password)
	if len(salt) > 0 {
		input = append(salt, input...)
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), input) == nil
}

func generateSalt() ([]byte, error) {
	b := make([]byte, saltSize)
	_, err := rand.Read(b)
	return b, err
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var user models.User
	err := h.db.Where("email = ?", req.Email).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		// Allow admin credentials from config (backward compat); ensure admin user exists
		if req.Email == h.cfg.AdminEmail && req.Password == h.cfg.AdminPassword {
			user, err = h.ensureAdminUser()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create admin user"})
				return
			}
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	} else {
		if !verifyPassword(user.PasswordHash, user.Salt, req.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
	}

	token, err := h.newToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user_id": user.ID})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var existing models.User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	salt, err := generateSalt()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
		return
	}
	hash, err := hashPasswordWithSalt(salt, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
		return
	}

	user := models.User{
		Email:        req.Email,
		Salt:         salt,
		PasswordHash: string(hash),
	}
	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
		return
	}

	token, err := h.newToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to sign token"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token, "user_id": user.ID})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var user models.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if !verifyPassword(user.PasswordHash, user.Salt, req.CurrentPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid current password"})
		return
	}
	salt, err := generateSalt()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}
	hash, err := hashPasswordWithSalt(salt, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}
	user.Salt = salt
	user.PasswordHash = string(hash)
	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

func (h *AuthHandler) ensureAdminUser() (models.User, error) {
	var u models.User
	if err := h.db.Where("email = ?", h.cfg.AdminEmail).First(&u).Error; err == nil {
		return u, nil
	}
	salt, err := generateSalt()
	if err != nil {
		return models.User{}, err
	}
	hash, err := hashPasswordWithSalt(salt, h.cfg.AdminPassword)
	if err != nil {
		return models.User{}, err
	}
	u = models.User{
		Email:        h.cfg.AdminEmail,
		Salt:         salt,
		PasswordHash: string(hash),
	}
	if err := h.db.Create(&u).Error; err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (h *AuthHandler) newToken(userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(h.cfg.JWTSecret))
}
