package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

var db *pgx.Conn

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error: Error loading .env file: ", err)
	}
}

func main() {
	r := gin.Default()

	// Connect to the database
	dbConnection()

	// Test route
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Pong!"})
	})

	// Add a new song
	r.POST("/addSong", addSong)

	// Get all songs
	r.GET("/getSongs", getSongs)

	// Start the server on port 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Println("Server running on port", port)
	r.Run(":" + port)
}

func dbConnection() {
	databaseUser := os.Getenv("DBUSER")
	databasePassword := os.Getenv("DBPASSWORD")
	databaseName := os.Getenv("DBNAME")
	databaseHost := os.Getenv("DBHOST")
	databasePort := os.Getenv("DBPORT")

	databaseURL := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", databaseUser, databasePassword, databaseHost, databasePort, databaseName)
	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("error: Unable to connect to database: %v: ", err)
	}
	db = conn
	fmt.Println("Connected to database successfully")
}

// adds a new song to the database
func addSong(c *gin.Context) {
	var song struct {
		SongID string `json:"song_id"`
		Title  string `json:"title"`
		Artist string `json:"artist"`
	}

	// Defines the structure of the json request to the song struct
	if err := c.ShouldBindJSON(&song); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "error: Invalid request: " + err.Error()})
		return
	}

	// Insert song into the database
	_, err := db.Exec(context.Background(),
		"INSERT INTO songs (song_id, title, artist) VALUES ($1, $2, $3)",
		song.SongID, song.Title, song.Artist)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error: Failed to insert song: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Song added successfully!"})
}

// retrieves all songs from the database
func getSongs(c *gin.Context) {
	rows, err := db.Query(context.Background(), "SELECT song_id, title, artist FROM songs")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error: Failed to retrieve songs: " + err.Error()})
		return
	}
	defer rows.Close()

	var songs []struct {
		SongID string `json:"song_id"`
		Title  string `json:"title"`
		Artist string `json:"artist"`
	}

	for rows.Next() {
		var song struct {
			SongID string `json:"song_id"`
			Title  string `json:"title"`
			Artist string `json:"artist"`
		}
		if err := rows.Scan(&song.SongID, &song.Title, &song.Artist); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error: Failed to retrieve songs: " + err.Error()})
			return
		}
		songs = append(songs, song)
	}

	// Check if there were any errors during the iteration
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error: Failed to iterate over songs: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"songs": songs})
}
