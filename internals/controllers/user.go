package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/waltertaya/cash-flow-forecast-backend/internals/db"
	"github.com/waltertaya/cash-flow-forecast-backend/internals/helpers"
	"github.com/waltertaya/cash-flow-forecast-backend/internals/models"
)

func setAuthCookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		MaxAge:   maxAge,
		Path:     "/",
		Domain:   "", // empty = no domain restriction
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode, // required for cross-origin
	})
}

func SignUp(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existingUser models.User
	if err := db.DB.Where("email = ?", input.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	user := models.User{
		Email:        input.Email,
		PasswordHash: helpers.HashPassword(input.Password),
	}

	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	token, err := helpers.GenerateJWT(user.ID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	setAuthCookie(c, token, 3600*24)

	// Return user so the frontend can populate auth state immediately
	c.JSON(http.StatusCreated, gin.H{
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
		},
	})
}

func Login(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := db.DB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	if !helpers.CheckPasswordHash(input.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	token, err := helpers.GenerateJWT(user.ID.String())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	setAuthCookie(c, token, 3600*24)

	// Return user so the frontend can populate auth state immediately
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
		},
	})
}

func Logout(c *gin.Context) {
	// MaxAge=-1 tells the browser to delete the cookie immediately.
	// Must match the same Path, Domain, Secure, SameSite as when it was set.
	setAuthCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	var user models.User
	if err := db.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		return
	}

	// Consistent envelope so frontend extractUser() works the same as login/signup
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":    user.ID,
			"email": user.Email,
		},
	})
}
