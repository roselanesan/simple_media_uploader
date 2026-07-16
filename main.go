package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"rosela.id/uas/controllers"
	"rosela.id/uas/middleware"
)

func main() {
	db := Koneksi()
	defer db.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	auth := controllers.NewAuthHandler(db, jwtSecret)
	media := controllers.NewMediaHandler(db)

	r := gin.Default()
	r.LoadHTMLGlob("web/*")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	api := r.Group("/api")
	{
		api.POST("/register", auth.Register)
		api.POST("/login", auth.Login)

		authGroup := api.Group("")
		authGroup.Use(middleware.JWTAuth(jwtSecret))
		{
			authGroup.POST("/upload", media.DirectUpload)
			authGroup.POST("/upload/presigned", media.CreatePresignedURL)
			authGroup.POST("/upload/confirm", media.ConfirmUpload)
			authGroup.GET("/media", media.ListMedia)
			authGroup.GET("/media/:id/url", media.GetMediaURL)
			authGroup.GET("/media/:id/public-url", media.GetMediaDirectURL)
			authGroup.DELETE("/media/:id", media.DeleteMedia)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "33080"
	}
	log.Println("Server berjalan di port", port)
	r.Run(":" + port)
}
