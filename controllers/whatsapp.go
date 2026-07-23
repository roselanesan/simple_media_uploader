package controllers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"rosela.id/uas/services/whatsapp"
)

type WhatsAppHandler struct {
	DB     *sql.DB
	Client *whatsapp.Client
}

func NewWhatsAppHandler(db *sql.DB, client *whatsapp.Client) *WhatsAppHandler {
	return &WhatsAppHandler{DB: db, Client: client}
}

func (h *WhatsAppHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": gin.H{
			"online": h.Client.IsLoggedIn(),
		},
	})
}

func (h *WhatsAppHandler) ListWhitelist(c *gin.Context) {
	rows, err := h.DB.Query("SELECT id, phone_number, created_at FROM whatsapp_whitelist ORDER BY created_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": err.Error()})
		return
	}
	defer rows.Close()

	type Item struct {
		ID          int    `json:"id"`
		PhoneNumber string `json:"phone_number"`
		CreatedAt   string `json:"created_at"`
	}

	var items []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.PhoneNumber, &it.CreatedAt); err != nil {
			continue
		}
		items = append(items, it)
	}

	if items == nil {
		items = []Item{}
	}

	c.JSON(http.StatusOK, gin.H{"status": true, "data": items})
}

func (h *WhatsAppHandler) AddWhitelist(c *gin.Context) {
	var input struct {
		PhoneNumber string `json:"phone_number" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "pesan": err.Error()})
		return
	}

	normalized := whatsapp.NormalizePhone(input.PhoneNumber)

	res, err := h.DB.Exec("INSERT INTO whatsapp_whitelist (phone_number) VALUES (?)", normalized)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": false, "pesan": "Nomor sudah ada atau tidak valid"})
		return
	}

	id, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{
		"status": true,
		"pesan":  "Nomor berhasil ditambahkan",
		"data": gin.H{
			"id":           id,
			"phone_number": normalized,
		},
	})
}

func (h *WhatsAppHandler) DeleteWhitelist(c *gin.Context) {
	phone := c.Param("phone")
	phone = whatsapp.NormalizePhone(phone)
	_, err := h.DB.Exec("DELETE FROM whatsapp_whitelist WHERE phone_number = ?", phone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": false, "pesan": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"pesan":  "Nomor berhasil dihapus",
	})
}
