package main

import (
	"database/sql"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

// Album represents an album entity
type Album struct {
	ID     int    `json:"id,omitempty"`
	Artist string `json:"artist"`
	Title  string `json:"title"`
	Year   int    `json:"year"`
	Image  []byte `json:"image,omitempty"`
}

// Global DB instance
var db *sql.DB

func initDB() {
	// Read MySQL DSN from environment variable
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN environment variable not set")
	}

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}

	// Test the DB connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}

	// Set connection pooling configurations
	db.SetMaxOpenConns(88)
	db.SetMaxIdleConns(30)
	db.SetConnMaxLifetime(0)

	// Create Albums table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS Albums (
			id INT AUTO_INCREMENT PRIMARY KEY,
			artist VARCHAR(255) NOT NULL,
			year INT NOT NULL,
			title VARCHAR(255) NOT NULL,
			image MEDIUMBLOB NOT NULL
		) ENGINE=InnoDB;
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
}

// CreateAlbum handles album creation
func createAlbum(c *gin.Context) {
	// Parse multipart form data
	err := c.Request.ParseMultipartForm(10 << 20) // 10MB limit
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}

	artist := c.Request.FormValue("artist")
	title := c.Request.FormValue("title")
	yearStr := c.Request.FormValue("year")

	// Validate required fields
	if artist == "" || title == "" || yearStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Artist, title, and year are required"})
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil || year <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Year must be a positive integer"})
		return
	}

	// Read image file
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image is required"})
		return
	}

	openedFile, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process image file"})
		return
	}
	defer openedFile.Close()

	imageData, err := io.ReadAll(openedFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read image file"})
		return
	}

	// Insert into database
	query := "INSERT INTO Albums (artist, title, year, image) VALUES (?, ?, ?, ?)"
	result, err := db.Exec(query, artist, title, year, imageData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert album"})
		return
	}

	albumID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve album ID"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"AlbumID": albumID})
}

// GetAlbum handles album retrieval
func getAlbum(c *gin.Context) {
	albumID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid album ID"})
		return
	}

	var album Album
	query := "SELECT id, artist, title, year, image FROM Albums WHERE id = ?"
	err = db.QueryRow(query, albumID).Scan(&album.ID, &album.Artist, &album.Title, &album.Year, &album.Image)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Album not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, album)
}

func main() {
	initDB()
	defer db.Close()

	// Setup Gin engine
	r := gin.Default()

	// Health check route
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Album routes
	r.POST("/albums", createAlbum)
	r.GET("/albums/:id", getAlbum)

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s ...", port)
	r.Run(":" + port)
}
