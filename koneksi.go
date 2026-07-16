package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func Koneksi() *sql.DB {
	err := godotenv.Load()
	if err != nil {
		log.Println("Info: .env tidak ditemukan, menggunakan environment variable sistem")
	}

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")

	dsn := dbUser + ":" + dbPass + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Gagal membuka koneksi database: ", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal("Gagal ping ke database: ", err)
	}

	return db
}
