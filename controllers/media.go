package controllers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

type MediaHandler struct {
	DB       *sql.DB
	S3Client *s3.Client
	Bucket   string
}

func NewMediaHandler(db *sql.DB) *MediaHandler {
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
		fmt.Println("Gagal load R2 config:", err)
	}

	r2Endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", os.Getenv("R2_ACCOUNT_ID"))

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(r2Endpoint)
		o.UsePathStyle = true
	})

	return &MediaHandler{
		DB:       db,
		S3Client: s3Client,
		Bucket:   os.Getenv("R2_BUCKET"),
	}
}

func (h *MediaHandler) CreatePresignedURL(c *gin.Context) {
	var input struct {
		Filename string `json:"filename" binding:"required"`
		MimeType string `json:"mime_type" binding:"required"`
		Size     int64  `json:"size" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "pesan": err.Error()})
		return
	}

	userID := c.GetInt("user_id")

	ext := ""
	if idx := strings.LastIndex(input.Filename, "."); idx != -1 {
		ext = input.Filename[idx:]
	}
	r2Key := fmt.Sprintf("uploads/%d/%d%s", userID, time.Now().UnixMilli(), ext)

	svc := s3.NewPresignClient(h.S3Client)
	presigned, err := svc.PresignPutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(h.Bucket),
		Key:         aws.String(r2Key),
		ContentType: aws.String(input.MimeType),
	}, func(o *s3.PresignOptions) {
		o.Expires = time.Duration(15) * time.Minute
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": "Gagal generate presigned URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": gin.H{
			"presigned_url": presigned.URL,
			"r2_key":        r2Key,
			"expires_in":    900,
		},
	})
}

func (h *MediaHandler) ConfirmUpload(c *gin.Context) {
	var input struct {
		R2Key    string `json:"r2_key" binding:"required"`
		Filename string `json:"filename" binding:"required"`
		MimeType string `json:"mime_type" binding:"required"`
		Size     int64  `json:"size" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "pesan": err.Error()})
		return
	}

	userID := c.GetInt("user_id")

	res, err := h.DB.Exec(
		"INSERT INTO media (user_id, filename, r2_key, mime_type, size, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		userID, input.Filename, input.R2Key, input.MimeType, input.Size, time.Now(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": "Gagal simpan metadata"})
		return
	}

	id, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"status": true,
		"pesan":  "Upload berhasil dikonfirmasi",
		"data": gin.H{
			"id":       id,
			"filename": input.Filename,
			"r2_key":   input.R2Key,
		},
	})
}

func (h *MediaHandler) ListMedia(c *gin.Context) {
	userID := c.GetInt("user_id")

	rows, err := h.DB.Query(
		"SELECT id, user_id, filename, r2_key, mime_type, size, created_at FROM media WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": err.Error()})
		return
	}
	defer rows.Close()

	type MediaItem struct {
		ID        int    `json:"id"`
		UserID    int    `json:"user_id"`
		Filename  string `json:"filename"`
		R2Key     string `json:"r2_key"`
		MimeType  string `json:"mime_type"`
		Size      int64  `json:"size"`
		CreatedAt string `json:"created_at"`
	}

	var mediaList []MediaItem
	for rows.Next() {
		var m MediaItem
		var createdAt time.Time
		if err := rows.Scan(&m.ID, &m.UserID, &m.Filename, &m.R2Key, &m.MimeType, &m.Size, &createdAt); err != nil {
			continue
		}
		m.CreatedAt = createdAt.Format(time.RFC3339)
		mediaList = append(mediaList, m)
	}

	if mediaList == nil {
		mediaList = []MediaItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data":   mediaList,
	})
}

func (h *MediaHandler) GetMediaURL(c *gin.Context) {
	userID := c.GetInt("user_id")
	mediaID := c.Param("id")

	var r2Key string
	err := h.DB.QueryRow(
		"SELECT r2_key FROM media WHERE id = ? AND user_id = ?",
		mediaID, userID,
	).Scan(&r2Key)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "pesan": "File tidak ditemukan"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": err.Error()})
		return
	}

	svc := s3.NewPresignClient(h.S3Client)
	presigned, err := svc.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(r2Key),
	}, func(o *s3.PresignOptions) {
		o.Expires = time.Duration(1) * time.Hour
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": "Gagal generate URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": gin.H{
			"url":       presigned.URL,
			"expires_in": 3600,
		},
	})
}

func (h *MediaHandler) GetMediaDirectURL(c *gin.Context) {
	userID := c.GetInt("user_id")
	mediaID := c.Param("id")

	publicURL := os.Getenv("R2_PUBLIC_URL")
	if publicURL == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": "R2_PUBLIC_URL belum dikonfigurasi"})
		return
	}

	var r2Key string
	err := h.DB.QueryRow(
		"SELECT r2_key FROM media WHERE id = ? AND user_id = ?",
		mediaID, userID,
	).Scan(&r2Key)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "pesan": "File tidak ditemukan"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": gin.H{
			"url": publicURL + "/" + r2Key,
		},
	})
}

func (h *MediaHandler) DeleteMedia(c *gin.Context) {
	userID := c.GetInt("user_id")
	mediaID := c.Param("id")

	var r2Key string
	err := h.DB.QueryRow(
		"SELECT r2_key FROM media WHERE id = ? AND user_id = ?",
		mediaID, userID,
	).Scan(&r2Key)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "pesan": "File tidak ditemukan"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": err.Error()})
		return
	}

	_, err = h.S3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(h.Bucket),
		Key:    aws.String(r2Key),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": "Gagal hapus file dari R2"})
		return
	}

	_, err = h.DB.Exec("DELETE FROM media WHERE id = ? AND user_id = ?", mediaID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": "Gagal hapus metadata"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"pesan":  "File berhasil dihapus",
	})
}
