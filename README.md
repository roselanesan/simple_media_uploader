# Media Uploader (Cloudflare R2 + Gin)

Backend API + dashboard untuk upload media ke Cloudflare R2 dengan presigned URL pattern.
Dibuat untuk UAS Golang - integrasi Object Storage pihak ke-3.

## Stack
- **Backend**: Go + Gin Framework
- **Database**: MySQL (user & metadata)
- **Object Storage**: Cloudflare R2 (S3-compatible)
- **Auth**: JWT (HS256)
- **Frontend**: HTML + JS native (no framework)

## Struktur
```
uas/
├── main.go              # entry point + routing
├── koneksi.go           # MySQL connection
├── controllers/
│   ├── auth.go          # register + login
│   └── media.go         # presigned URL, upload, list, delete
├── middleware/
│   └── auth.go          # JWT middleware
├── models/
│   └── models.go        # User, Media structs
├── web/
│   └── index.html       # dashboard HTML/JS
├── schema.sql           # MySQL schema
└── .env.example
```

## Setup Lokal

1. **Clone & install dependencies**
   ```bash
   cd uas
   go mod tidy
   ```

2. **Buat database**
   ```bash
   mysql -u root -p < schema.sql
   ```

3. **Setup R2**
   - Buat bucket di Cloudflare R2
   - Generate API Token dengan permission Object Read & Write
   - Catat: Account ID, Access Key ID, Secret Access Key, Bucket Name
   - (Opsional) Aktifkan public bucket atau custom domain → set R2_PUBLIC_URL

4. **Copy .env**
   ```bash
   cp .env.example .env
   # Isi semua nilai sesuai setup R2 & MySQL kamu
   ```

5. **Run**
   ```bash
   go run .
   ```
   Server jalan di `http://localhost:8080`

## Deployment ke Railway

1. Push repo ke GitHub
2. Buka railway.app → New Project → Deploy from GitHub
3. Pilih repo ini, set root directory: `uas`
4. Tambah MySQL service di Railway (atau pakai MySQL external)
5. Set environment variables di Railway sama seperti `.env`
6. Railway auto-detect Go → langsung build & deploy

## Alur Upload (Presigned URL)
1. Frontend request presigned URL → backend
2. Backend generate URL → return ke frontend
3. Frontend upload file langsung ke R2 (tidak lewat backend)
4. Frontend konfirmasi ke backend → simpan metadata ke MySQL
5. List media dari MySQL, preview via presigned GET URL

## Catatan
- File benar-benar tersimpan di Cloudflare R2 (cek dashboard R2)
- Database MySQL hanya menyimpan metadata (id, user, filename, r2_key, dll)
- `ponytail:` Presigned URL expire 15 menit (upload) & 1 jam (view). Cukup untuk demo, tambah waktu jika production.
