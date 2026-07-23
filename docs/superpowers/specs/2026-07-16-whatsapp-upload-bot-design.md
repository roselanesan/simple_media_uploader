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

## Out of Scope
- Tidak menggabungkan fitur AI dari `whatsapp-ai`.
- Tidak ada registrasi/mapping nomor WA ke user.
- Tidak ada dashboard QR scan; QR tetap di terminal.

## Architecture

```
WhatsApp Message (image/video/document)
        ↓
whatsmeow Client
        ↓
WA Bot Handler
        ↓
Download media bytes
        ↓
S3Client.PutObject → Cloudflare R2
        ↓
INSERT INTO media (user_id, filename, r2_key, mime_type, size, source, created_at)
        ↓
Send public URL back to WhatsApp user
```

## Database Changes

Tambah kolom `source` di tabel `media`:

```sql
ALTER TABLE media ADD COLUMN source VARCHAR(50) DEFAULT 'web';
```

File dari WhatsApp akan memiliki `source = 'whatsapp'`.

## New Components

### 1. `services/whatsapp/client.go`
Adaptasi dari `whatsapp-ai/internal/whatsapp/client.go`:
- `NewClient(dbPath string)`
- `Login(ctx)` — QR terminal login
- `Start(ctx, handler)` — event handler untuk message
- `SendText(ctx, chat, text)` — balas ke user
- `DownloadMedia(ctx, message)` — ambil bytes dari media message

### 2. `services/whatsapp/bot.go`
Logic handler message:
- Filter message dengan media (`ImageMessage`, `VideoMessage`, `DocumentMessage`).
- Download bytes + filename + mimetype + size.
- Upload ke R2: key pattern `uploads/wa/<timestamp><ext>`.
- Simpan metadata ke MySQL.
- Balas user dengan URL.

### 3. `main.go` Changes
- Inisialisasi WA bot setelah DB & R2 client siap.
- Start WA bot di goroutine.
- Gunakan shared `signal.NotifyContext` untuk graceful shutdown.

### 4. ENV Variables

Tambah di `.env.example`:

```
WHATSAPP_ENABLED=true
WHATSAPP_DB_PATH=store/whatsapp.db
```

## API Endpoints (Optional)

| Method | Endpoint | Auth | Fungsi |
|--------|----------|------|--------|
| GET | `/api/whatsapp/status` | ✓ | Status WA bot online/offline |

## File List Changes

| File | Action | Description |
|------|--------|-------------|
| `schema.sql` | Modify | Add `source` column |
| `main.go` | Modify | Start WA bot goroutine |
| `.env.example` | Modify | Add WA env vars |
| `README.md` | Modify | Dokumentasi WhatsApp upload |
| `services/whatsapp/client.go` | Create | whatsmeow wrapper |
| `services/whatsapp/bot.go` | Create | upload handler logic |

## Testing Checklist
- [ ] Bot bisa login dengan QR scan.
- [ ] Kirim gambar dari WA → file muncul di R2 dashboard.
- [ ] Metadata tersimpan di MySQL dengan `source = 'whatsapp'`.
- [ ] Bot membalas dengan public URL yang bisa dibuka.
- [ ] Dashboard web tetap bisa upload file normal.
- [ ] Graceful shutdown tidak crash.

## Deployment Notes
- `WHATSAPP_DB_PATH` harus persistent di Railway (gunakan Volume jika perlu, atau path writable).
- QR scan hanya pertama kali; session tersimpan di SQLite `store/whatsapp.db`.
- Pastikan R2 public URL masih sama dengan konfigurasi yang sudah ada.
