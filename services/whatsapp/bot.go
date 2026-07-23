package whatsapp

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

type Bot struct {
	db           *sql.DB
	waClient     *Client
	s3Client     *s3.Client
	bucket       string
	publicURL    string
	aiClient     AIClient
	uploadPrefix string
}

type AIClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

func NewBot(db *sql.DB, waClient *Client, s3Client *s3.Client, bucket, publicURL string, aiClient AIClient, uploadPrefix string) *Bot {
	return &Bot{
		db:           db,
		waClient:     waClient,
		s3Client:     s3Client,
		bucket:       bucket,
		publicURL:    publicURL,
		aiClient:     aiClient,
		uploadPrefix: uploadPrefix,
	}
}

func (b *Bot) Handle(ctx context.Context, sender string, chat string, evt *events.Message) (string, error) {
	if !b.isWhitelisted(ctx, sender) {
		return "", nil
	}

	text := getMessageText(evt.Message)
	text = strings.TrimSpace(text)

	hasMed := hasMedia(evt.Message)
	if text != "" && strings.HasPrefix(text, b.uploadPrefix) {
		return b.handleUpload(ctx, evt)
	}

	if hasMed {
		return b.handleUpload(ctx, evt)
	}

	if text == "" {
		return "", nil
	}

	return b.handleAIChat(ctx, text)
}

func (b *Bot) isWhitelisted(ctx context.Context, phone string) bool {
	phone = normalizePhone(phone)
	var id int
	err := b.db.QueryRowContext(ctx, "SELECT id FROM whatsapp_whitelist WHERE phone_number = ?", phone).Scan(&id)
	if err != nil {
		slog.Debug("whitelist check", "phone", phone, "allowed", false, "error", err)
		return false
	}
	return true
}

func (b *Bot) handleUpload(ctx context.Context, evt *events.Message) (string, error) {
	if evt.Message == nil {
		return "Kirim gambar/video/dokumen dengan caption !upload", nil
	}

	var (
		data     []byte
		filename string
		mimeType string
		err      error
	)

	if img := evt.Message.GetImageMessage(); img != nil {
		data, err = b.waClient.Download(ctx, img)
		filename = fmt.Sprintf("wa_image_%d.jpg", time.Now().UnixMilli())
		if caption := img.GetCaption(); caption != "" {
			filename = caption
		}
		mimeType = img.GetMimetype()
	} else if vid := evt.Message.GetVideoMessage(); vid != nil {
		data, err = b.waClient.Download(ctx, vid)
		filename = fmt.Sprintf("wa_video_%d.mp4", time.Now().UnixMilli())
		if caption := vid.GetCaption(); caption != "" {
			filename = caption
		}
		mimeType = vid.GetMimetype()
	} else if doc := evt.Message.GetDocumentMessage(); doc != nil {
		data, err = b.waClient.Download(ctx, doc)
		filename = doc.GetFileName()
		if filename == "" {
			filename = fmt.Sprintf("wa_doc_%d", time.Now().UnixMilli())
		}
		mimeType = doc.GetMimetype()
	} else {
		return "Kirim gambar/video/dokumen dengan caption !upload", nil
	}

	if err != nil {
		slog.Error("download media error", "error", err)
		return "Gagal download media.", nil
	}

	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	ext := ""
	if idx := strings.LastIndex(filename, "."); idx != -1 {
		ext = filename[idx:]
	}
	if ext == "" {
		ext = mimeToExt(mimeType)
	}
	r2Key := fmt.Sprintf("uploads/wa/%d%s", time.Now().UnixMilli(), ext)

	_, err = b.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(r2Key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(mimeType),
	})
	if err != nil {
		slog.Error("R2 PutObject error", "error", err)
		return "Gagal upload ke storage.", nil
	}

	size := int64(len(data))
	userID := 1

	_, err = b.db.ExecContext(ctx,
		"INSERT INTO media (user_id, filename, r2_key, mime_type, size, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		userID, path.Base(filename), r2Key, mimeType, size, "whatsapp", time.Now(),
	)
	if err != nil {
		slog.Error("db insert error", "error", err)
		return "Upload berhasil tapi gagal simpan metadata.", nil
	}

	publicURL := b.publicURL
	if publicURL == "" {
		publicURL = fmt.Sprintf("https://%s.r2.cloudflarestorage.com/%s", os.Getenv("R2_ACCOUNT_ID"), b.bucket)
	}

	url := publicURL + "/" + r2Key
	response := fmt.Sprintf(
		"Upload berhasil!\n\nNama: %s\nUkuran: %s\nURL: %s",
		path.Base(filename),
		formatSize(size),
		url,
	)
	return response, nil
}

func (b *Bot) handleAIChat(ctx context.Context, text string) (string, error) {
	if b.aiClient == nil {
		return "AI chat tidak aktif.", nil
	}

	response, err := b.aiClient.Generate(ctx, text)
	if err != nil {
		slog.Error("AI error", "error", err)
		return "AI sedang tidak tersedia, coba lagi nanti.", nil
	}
	return response, nil
}

func getMessageText(m *proto.Message) string {
	if text := m.GetConversation(); text != "" {
		return text
	}
	if extended := m.GetExtendedTextMessage(); extended != nil {
		return extended.GetText()
	}
	return ""
}

func hasMedia(m *proto.Message) bool {
	return m.GetImageMessage() != nil || m.GetVideoMessage() != nil || m.GetDocumentMessage() != nil
}

func normalizePhone(phone string) string {
	phone = strings.TrimPrefix(phone, "+")
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	return phone
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	if bytes < 1048576 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/1048576)
}

func mimeToExt(mime string) string {
	switch {
	case strings.Contains(mime, "image/jpeg"):
		return ".jpg"
	case strings.Contains(mime, "image/png"):
		return ".png"
	case strings.Contains(mime, "image/gif"):
		return ".gif"
	case strings.Contains(mime, "video/mp4"):
		return ".mp4"
	case strings.Contains(mime, "audio/"):
		return ".ogg"
	case strings.Contains(mime, "application/pdf"):
		return ".pdf"
	case strings.Contains(mime, "image/webp"):
		return ".webp"
	default:
		return ".bin"
	}
}
