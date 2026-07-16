package controllers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB        *sql.DB
	JWTSecret string
}

func NewAuthHandler(db *sql.DB, secret string) *AuthHandler {
	return &AuthHandler{DB: db, JWTSecret: secret}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "pesan": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": "Gagal hash password"})
		return
	}

	res, err := h.DB.Exec(
		"INSERT INTO users (username, password, created_at) VALUES (?, ?, ?)",
		input.Username, string(hash), time.Now(),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "pesan": "Username sudah dipakai"})
		return
	}

	id, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"status": true,
		"pesan":  "Registrasi berhasil",
		"data": gin.H{
			"id":       id,
			"username": input.Username,
		},
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "pesan": err.Error()})
		return
	}

	var (
		id       int
		username string
		hash     string
	)
	err := h.DB.QueryRow(
		"SELECT id, username, password FROM users WHERE username = ?",
		input.Username,
	).Scan(&id, &username, &hash)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "pesan": "Username atau password salah"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": err.Error()})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status": false, "pesan": "Username atau password salah"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  id,
		"username": username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": "Gagal buat token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"pesan":  "Login berhasil",
		"data": gin.H{
			"token": tokenString,
			"user": gin.H{
				"id":       id,
				"username": username,
			},
		},
	})
}
