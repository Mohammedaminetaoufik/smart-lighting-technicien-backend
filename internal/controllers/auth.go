package controllers

import (
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"technicien-mobile/internal/middleware"
	"technicien-mobile/internal/repository"
)

func HandleLogin(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Email    string `json:"email"    binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email et mot de passe requis"})
			return
		}

		user, err := repository.FindUserByEmail(c.Request.Context(), db, body.Email)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "identifiants invalides"})
			return
		}
		if user.Status != "active" {
			c.JSON(http.StatusForbidden, gin.H{"error": "compte désactivé"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "identifiants invalides"})
			return
		}

		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration serveur invalide"})
			return
		}

		claims := middleware.AuthClaims{
			Sub:   strconv.Itoa(user.ID),
			Name:  user.FullName,
			Email: user.Email,
			Role:  user.Role,
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			},
		}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "génération du token impossible"})
			return
		}

		_ = repository.UpdateLastLogin(c.Request.Context(), db, user.ID)

		c.JSON(http.StatusOK, gin.H{
			"token": token,
			"user": gin.H{
				"id":    user.ID,
				"name":  user.FullName,
				"email": user.Email,
				"role":  user.Role,
			},
		})
	}
}
