package utils

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)


func RespondJSON(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

func RespondError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func ParseIDParam(c *gin.Context, name string) (int, error) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("identifiant invalide")
	}
	return id, nil
}

// GetTestTechnicianID returns the authenticated technician ID from the JWT context
// (set by RequireAuth middleware), falling back to query param or header for testing.
func GetTestTechnicianID(c *gin.Context) (int, string) {
	// JWT context — primary source when middleware is active
	if v, exists := c.Get("user_id"); exists {
		if id, err := strconv.Atoi(fmt.Sprint(v)); err == nil && id > 0 {
			return id, "jwt"
		}
	}
	if v := c.Query("technician_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil && id > 0 {
			return id, "query"
		}
	}
	if v := c.GetHeader("X-Test-Technician-Id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil && id > 0 {
			return id, "header"
		}
	}
	if v := os.Getenv("DEFAULT_TECHNICIAN_ID"); v != "" {
		if id, err := strconv.Atoi(v); err == nil && id > 0 {
			return id, "env"
		}
	}
	return 1, "default"
}

func AuthEnabled() bool {
	return os.Getenv("AUTH_ENABLED") == "true"
}

func RespondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

func RespondCreated(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, data)
}
