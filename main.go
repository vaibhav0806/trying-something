package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const codeAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const codeLength = 6

type URLRecord struct {
	Code      string    `json:"code"`
	URL       string    `json:"url"`
	Hits      int       `json:"hits"`
	CreatedAt time.Time `json:"created_at"`
}

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Code string `json:"code"`
}

type StatsResponse struct {
	Code      string    `json:"code"`
	URL       string    `json:"url"`
	Hits      int       `json:"hits"`
	CreatedAt time.Time `json:"created_at"`
}

var db *sql.DB

func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "urls.db")
	if err != nil {
		return nil, err
	}

	query := `
	CREATE TABLE IF NOT EXISTS urls (
		code TEXT PRIMARY KEY,
		url TEXT NOT NULL,
		hits INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func generateCode() (string, error) {
	code := make([]byte, codeLength)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		if err != nil {
			return "", err
		}
		code[i] = codeAlphabet[n.Int64()]
	}
	return string(code), nil
}

func shortenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	code, err := generateCode()
	if err != nil {
		http.Error(w, "Failed to generate code", http.StatusInternalServerError)
		return
	}

	// Retry if code collides
	for {
		_, err = db.Exec("INSERT INTO urls (code, url) VALUES (?, ?)", code, req.URL)
		if err != nil {
			if isUniqueViolation(err) {
				code, err = generateCode()
				if err != nil {
					http.Error(w, "Failed to generate code", http.StatusInternalServerError)
					return
				}
				continue
			}
			http.Error(w, "Failed to save URL", http.StatusInternalServerError)
			return
		}
		break
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ShortenResponse{Code: code})
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		http.Error(w, "Code not provided", http.StatusBadRequest)
		return
	}

	var url string
	err := db.QueryRow("SELECT url FROM urls WHERE code = ?", code).Scan(&url)
	if err == sql.ErrNoRows {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("UPDATE urls SET hits = hits + 1 WHERE code = ?", code)
	if err != nil {
		log.Printf("Failed to increment hits: %v", err)
	}

	http.Redirect(w, r, url, http.StatusFound)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		http.Error(w, "Code not provided", http.StatusBadRequest)
		return
	}

	var record StatsResponse
	err := db.QueryRow("SELECT code, url, hits, created_at FROM urls WHERE code = ?", code).Scan(
		&record.Code, &record.URL, &record.Hits, &record.CreatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(record)
}

func recentHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT code, url, hits, created_at FROM urls ORDER BY created_at DESC LIMIT 10")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var records []URLRecord
	for rows.Next() {
		var rec URLRecord
		if err := rows.Scan(&rec.Code, &rec.URL, &rec.Hits, &rec.CreatedAt); err != nil {
			continue
		}
		records = append(records, rec)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

func main() {
	var err error
	db, err = initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", shortenHandler)
	mux.HandleFunc("GET /stats/{code}", statsHandler)
	mux.HandleFunc("GET /recent", recentHandler)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/index.html")
	})
	mux.HandleFunc("GET /{code}", redirectHandler)

	port := "8080"
	fmt.Printf("Server running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
