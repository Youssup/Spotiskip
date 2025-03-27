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

	// Add skipped sections to a song
	r.POST("/addSkippedSections", addSkippedSections)

	// Get song by ID
	r.GET("/getSong/:id", getSong)

	// Get song with skipped sections by ID
	r.GET("/getSongDetails/:id", getSongDetails)

	// Get all songs
	r.GET("/getSongs", getSongs)

	// Update a song by ID
	r.PUT("/updateSong/:id", updateSong)

	// Delete a song by ID
	r.DELETE("/deleteSong/:id", deleteSong)

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

// Add skipped sections to a song
func addSkippedSections(c *gin.Context) {
	var request struct {
		SongID          string `json:"song_id"`
		SkippedSections []struct {
			ID        int `json:"id"`
			StartTime int `json:"start_time"`
			EndTime   int `json:"end_time"`
		} `json:"skipped_sections"`
	}

	// Bind JSON request to struct
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Check if the song exists before inserting skipped sections
	var exists bool
	err := db.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM songs WHERE song_id = $1)", request.SongID).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check if song exists: " + err.Error()})
		return
	}
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Song not found"})
		return
	}

	// Insert each skipped section into the database
	for _, section := range request.SkippedSections {
		_, err := db.Exec(context.Background(),
			"INSERT INTO skipped_sections (song_id, start_time, end_time) VALUES ($1, $2, $3)",
			request.SongID, section.StartTime, section.EndTime)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert skipped section: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Skipped sections added successfully!"})
}

// Adds a new song to the database
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

// Retrieves one song from the database
func getSong(c *gin.Context) {
	songID := c.Param("id")
	if songID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "error: Invalid song ID"})
		return
	}

	var song struct {
		SongID string `json:"song_id"`
		Title  string `json:"title"`
		Artist string `json:"artist"`
	}

	err := db.QueryRow(context.Background(), "SELECT song_id, title, artist FROM songs WHERE song_id = $1", songID).
		Scan(&song.SongID, &song.Title, &song.Artist)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error: Failed to retrieve song: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Song retrieved successfully!", "song": song})
}

// Retrieves a song with its skipped sections
func getSongDetails(c *gin.Context) {
	songID := c.Param("id")

	var song struct {
		SongID          string `json:"song_id"`
		Title           string `json:"title"`
		Artist          string `json:"artist"`
		SkippedSections []struct {
			ID        int `json:"id"`
			StartTime int `json:"start_time"`
			EndTime   int `json:"end_time"`
		} `json:"skipped_sections"`
	}

	err := db.QueryRow(context.Background(),
		"SELECT song_id, title, artist FROM songs WHERE song_id = $1", songID).
		Scan(&song.SongID, &song.Title, &song.Artist)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "error: Song not found"})
		return
	}

	rows, err := db.Query(context.Background(),
		"SELECT id, start_time, end_time, created_at FROM skipped_sections WHERE song_id = $1", songID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error: Failed to retrieve skipped sections"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var section struct {
			ID        int `json:"id"`
			StartTime int `json:"start_time"`
			EndTime   int `json:"end_time"`
		}
		if err := rows.Scan(&section.ID, &section.StartTime, &section.EndTime); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error: Failed to process skipped sections"})
			return
		}
		song.SkippedSections = append(song.SkippedSections, section)
	}

	c.JSON(http.StatusOK, song)
}

// Retrieves all songs from the database
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

	c.JSON(http.StatusOK, gin.H{
		"message": "Songs retrieved successfully!",
		"songs":   songs})
}

// Delete a song by ID
func deleteSong(c *gin.Context) {
	songID := c.Param("id")

	_, err := db.Exec(context.Background(),
		"DELETE FROM songs WHERE song_id = $1", songID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error: Failed to delete song: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Song deleted successfully!"})
}

// Update a song
func updateSong(c *gin.Context) {
	songID := c.Param("id")

	var song struct {
		Title  string `json:"title"`
		Artist string `json:"artist"`
	}

	if err := c.ShouldBindJSON(&song); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "error: Invalid request: " + err.Error()})
		return
	}

	_, err := db.Exec(context.Background(),
		"UPDATE songs SET title = $1, artist = $2 WHERE song_id = $3",
		song.Title, song.Artist, songID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error: Failed to update song: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Song updated successfully!"})
}
