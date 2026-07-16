package main

import (
	"database/sql"
	"log"
	"net/url"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func Koneksi() *sql.DB {
	err := godotenv.Load()
	if err != nil {
		log.Println("Info: .env tidak ditemukan, menggunakan environment variable sistem")
	}

	var dsn string
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		dsn = parseMySQLURL(dbURL)
	} else {
		dbHost := os.Getenv("DB_HOST")
		dbPort := os.Getenv("DB_PORT")
		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASS")
		dbName := os.Getenv("DB_NAME")
		dsn = dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?charset=utf8mb4&parseTime=True&loc=Local"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Gagal membuka koneksi database: ", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal("Gagal ping ke database: ", err)
	}

	return db
}

func parseMySQLURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		log.Fatal("DATABASE_URL tidak valid: ", err)
	}

	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Host
	dbName := strings.TrimPrefix(u.Path, "/")

	query := u.Query()
	if query.Get("replicaSet") != "" {
		query.Del("replicaSet")
	}
	if query.Get("tls") != "" {
		query.Del("tls")
	}
	q := query.Encode()

	dsn := user + ":" + pass + "@tcp(" + host + ")/" + dbName
	if q != "" {
		dsn += "?" + q
	}
	return dsn
}
