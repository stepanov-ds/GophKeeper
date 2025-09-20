package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stepanov-ds/GophKeeper/internal/config"
	"github.com/stepanov-ds/GophKeeper/internal/database"
	"github.com/stepanov-ds/GophKeeper/internal/mail"
	"github.com/stepanov-ds/GophKeeper/internal/utils"
	"github.com/stepanov-ds/GophKeeper/internal/utils/structs"
)

func LoginGet(c *gin.Context, cache *utils.MemoryCache) {
	var bodyJSON struct {
		Mail string `json:"mail"`
	}
	if err := c.ShouldBindBodyWithJSON(&bodyJSON); err != nil {
		err = fmt.Errorf("error while parsing JSON: %w", err)
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}

	if err := database.CheckUser(bodyJSON.Mail); err != nil {
		err = fmt.Errorf("error while checking user: %w", err)
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}

	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		err = fmt.Errorf("error while generating challenge string: %w", err)
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}
	challenge := hex.EncodeToString(bytes)

	cache.Set(bodyJSON.Mail, challenge, 5*time.Minute)

	mail.Send(bodyJSON.Mail, challenge)
	c.JSON(http.StatusOK, structs.Response{
		Message: challenge,
	})
}

func LoginPost(c *gin.Context, cache *utils.MemoryCache) {
	var bodyJSON struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindBodyWithJSON(&bodyJSON); err != nil {
		err = fmt.Errorf("error while parsing JSON: %w", err)
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}
	challenge, success := cache.Get(bodyJSON.Login)
	if !success {
		err := fmt.Errorf("no challenge in cache")
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}

	if challenge != bodyJSON.Password {
		err := fmt.Errorf("invalid challenge")
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}

	tokenString, err := generateJWT(bodyJSON.Login)
	if err != nil {
		err := fmt.Errorf("error while generating token: %w", err)
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("Authorization", tokenString, 86400, "", "", false, true)
	c.JSON(http.StatusOK, structs.Response{
		Message: "authorized",
	})
}

type Claims struct {
	jwt.RegisteredClaims
	Login string
}

func generateJWT(login string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Login: login,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(config.JWTKey)
}
