package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"rosela.id/uas/controllers"
	"rosela.id/uas/middleware"
	"rosela.id/uas/services/ai"
	"rosela.id/uas/services/whatsapp"
)

func main() {
	godotenv.Load()

	db := Koneksi()
	defer db.Close()

	r2Client := initR2()

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

		if os.Getenv("WHATSAPP_ENABLED") == "true" {
			waClient := initWhatsApp()
			aiClient := ai.NewClient(os.Getenv("WHATSAPP_AI_BASEURL"), os.Getenv("WHATSAPP_AI_MODEL"))
			bot := whatsapp.NewBot(db, waClient, r2Client, os.Getenv("R2_BUCKET"), os.Getenv("R2_PUBLIC_URL"), aiClient, os.Getenv("WHATSAPP_UPLOAD_PREFIX"))
			waHandler := controllers.NewWhatsAppHandler(db, waClient)

			authGroup.GET("/whatsapp/status", waHandler.Status)
			authGroup.GET("/whatsapp/whitelist", waHandler.ListWhitelist)
			authGroup.POST("/whatsapp/whitelist", waHandler.AddWhitelist)
			authGroup.DELETE("/whatsapp/whitelist/:phone", waHandler.DeleteWhitelist)

			go startWhatsApp(waClient, bot)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "33080"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		log.Println("Server berjalan di port", port)
		if err := r.Run(":" + port); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")
}

func initR2() *s3.Client {
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			os.Getenv("R2_ACCESS_KEY_ID"),
			os.Getenv("R2_SECRET_ACCESS_KEY"),
			"",
		)),
		awsconfig.WithRegion(os.Getenv("R2_REGION")),
	)
	if err != nil {
		log.Println("R2 config warning:", err)
	}

	r2Endpoint := "https://" + os.Getenv("R2_ACCOUNT_ID") + ".r2.cloudflarestorage.com"

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(r2Endpoint)
		o.UsePathStyle = true
	})
}

func initWhatsApp() *whatsapp.Client {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")

	dsn := dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?charset=utf8mb4&parseTime=True&loc=Local"
	return whatsapp.NewClient(dsn)
}

func startWhatsApp(waClient *whatsapp.Client, bot *whatsapp.Bot) {
	ctx := context.Background()
	if err := waClient.Login(ctx); err != nil {
		log.Println("WhatsApp login error:", err)
		return
	}
	defer waClient.Logout()

	if err := waClient.Start(ctx, bot.Handle); err != nil {
		log.Println("WhatsApp start error:", err)
	}
}
