package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": false, "pesan": "Token tidak ditemukan"})
			return
		}

		tokenString := strings.TrimPrefix(auth, "Bearer ")
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": false, "pesan": "Token tidak valid"})
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if uid, ok := claims["user_id"].(float64); ok {
				c.Set("user_id", int(uid))
			}
			if uname, ok := claims["username"].(string); ok {
				c.Set("username", uname)
			}
		}

		c.Next()
	}
}
