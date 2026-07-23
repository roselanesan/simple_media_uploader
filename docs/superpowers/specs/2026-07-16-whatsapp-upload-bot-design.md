# WhatsApp Upload Bot Integration Design

## Context
Project UAS Media Uploader (Go + Gin + Cloudflare R2 + MySQL) sudah memiliki:
- JWT-based login + dashboard web
- REST API untuk upload file ke R2 (multipart & presigned URL)
- MySQL tabel `users` dan `media` untuk metadata

Project terpisah `whatsapp-ai/` sudah memiliki bot WhatsApp berbasis `whatsmeow` yang menerima pesan teks dan membalas via AI.

## Goal
Integrasikan bot WhatsApp ke project UAS agar user dapat **mengupload media (gambar/video/dokumen) langsung dari WhatsApp ke Cloudflare R2**, lalu bot membalas dengan public URL file.

## Scope
- Upload media via WhatsApp bot → R2 → MySQL metadata.
- Bot membalas user dengan nama file, ukuran, dan public URL.
- File dari WA disimpan sebagai user khusus (`user_id = 1` / bot user).
- Bot berjalan parallel dengan API Gin di goroutine yang sama.
- AI chat bot untuk nomor whitelist via OpenAI-compatible API (9router).

## Out of Scope
- Tidak ada registrasi/mapping nomor WA ke user.
- Tidak ada dashboard QR scan; QR tetap di terminal.

## Architecture

### WhatsApp Upload Flow
```
WhatsApp Message: "!upload" + media attachment
        ↓
whatsmeow Client
        ↓
WA Bot Handler
        ↓
Whitelist check → if allowed
        ↓
Download media bytes
        ↓
S3Client.PutObject → Cloudflare R2
        ↓
INSERT INTO media (user_id, filename, r2_key, mime_type, size, source, created_at)
        ↓
Send public URL back to WhatsApp user
```

### AI Chat Flow
```
WhatsApp text message from whitelist number (no prefix needed)
        ↓
Send text to OpenAI-compatible API (9router)
        ↓
Send AI response back to WhatsApp user
```

## Database Changes

Tambah kolom `source` di tabel `media`:

```sql
ALTER TABLE media ADD COLUMN source VARCHAR(50) DEFAULT 'web';
```

File dari WhatsApp akan memiliki `source = 'whatsapp'`.

Tambah tabel whitelist:

```sql
CREATE TABLE whatsapp_whitelist (
    id INT PRIMARY KEY AUTO_INCREMENT,
    phone_number VARCHAR(50) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## New Components

### 1. `services/whatsapp/client.go`
Adaptasi dari `whatsapp-ai/internal/whatsapp/client.go`:
- `NewClient(dsn string)` — MySQL session storage via `whatsmeow/store/sqlstore`
- `Login(ctx)` — QR terminal login
- `Start(ctx, handler)` — event handler untuk message
- `SendText(ctx, chat, text)` — balas ke user
- `DownloadMedia(ctx, message)` — ambil bytes dari media message

### 2. `services/whatsapp/bot.go`
Logic handler message:
- **Whitelist check**: hanya proses nomor yang ada di `whatsapp_whitelist`.
- **Upload command**: jika pesan diawali `!upload` dan ada media attachment (`ImageMessage`, `VideoMessage`, `DocumentMessage`), download bytes + filename + mimetype + size, upload ke R2 dengan key `uploads/wa/<timestamp><ext>`, simpan metadata, balas URL.
- **AI chat**: jika pesan berupa teks biasa (bukan command), langsung kirim ke OpenAI-compatible API (9router), balas hasilnya.
- **Ignore**: semua pesan lain di-abaikan.

### 3. `services/ai/openai.go`
- `NewClient(baseURL, apiKey, model string)`
- `Generate(ctx, prompt string) (string, error)`
- Menggunakan `github.com/openai/openai-go` atau HTTP manual ke endpoint 9router.

### 4. `main.go` Changes
- Inisialisasi WA bot setelah DB & R2 client siap.
- Start WA bot di goroutine.
- Gunakan shared `signal.NotifyContext` untuk graceful shutdown.
- Bot jalan hanya jika `WHATSAPP_ENABLED=true`.

### 5. ENV Variables

Tambah di `.env.example`:

```
WHATSAPP_ENABLED=true
WHATSAPP_UPLOAD_PREFIX=!upload
WHATSAPP_AI_BASEURL=https://9router.roselaa.my.id/v1
WHATSAPP_AI_MODEL=qwen2.5:0.5b
WHATSAPP_AI_API_KEY=optional-api-key
```

## API Endpoints

| Method | Endpoint | Auth | Fungsi |
|--------|----------|------|--------|
| GET | `/api/whatsapp/status` | ✓ | Status WA bot online/offline |
| POST | `/api/whatsapp/whitelist` | ✓ | Tambah nomor ke whitelist |
| DELETE | `/api/whatsapp/whitelist/:phone` | ✓ | Hapus nomor dari whitelist |
| GET | `/api/whatsapp/whitelist` | ✓ | List nomor whitelist |

## File List Changes

| File | Action | Description |
|------|--------|-------------|
| `schema.sql` | Modify | Add `source` column & `whatsapp_whitelist` table |
| `main.go` | Modify | Start WA bot goroutine |
| `.env.example` | Modify | Add WA & AI env vars |
| `README.md` | Modify | Dokumentasi WhatsApp upload & AI chat |
| `services/whatsapp/client.go` | Create | whatsmeow wrapper dengan MySQL session |
| `services/whatsapp/bot.go` | Create | upload & AI handler logic |
| `services/ai/openai.go` | Create | OpenAI-compatible AI client |
| `controllers/whatsapp.go` | Create | REST API untuk whitelist management |

## Testing Checklist
- [ ] Bot bisa login dengan QR scan.
- [ ] Session tersimpan di MySQL (tabel whatsmeow otomatis dibuat).
- [ ] Nomor tidak di-whitelist tidak dibalas.
- [ ] Kirim `!upload` + gambar dari nomor whitelist → file muncul di R2 dashboard.
- [ ] Metadata tersimpan di MySQL dengan `source = 'whatsapp'`.
- [ ] Bot membalas dengan public URL yang bisa dibuka.
- [ ] Kirim `halo` dari nomor whitelist → bot membalas dari 9router.
- [ ] Dashboard web tetap bisa upload file normal.
- [ ] Graceful shutdown tidak crash.

## Deployment Notes
- WA session disimpan di MySQL yang sama dengan aplikasi, survive redeploy.
- QR scan hanya pertama kali deploy.
- Pastikan R2 public URL masih sama dengan konfigurasi yang sudah ada.
- Pastikan 9router URL dapat diakses dari Railway.
