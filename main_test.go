package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func setupTestDB(t *testing.T) func() {
	var err error
	db, err = initDB()
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	// Clean table for each test
	db.Exec("DELETE FROM urls")
	return func() {
		db.Close()
		os.Remove("urls.db")
	}
}

func TestShortenAndRedirect(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", shortenHandler)
	mux.HandleFunc("GET /{code}", redirectHandler)
	mux.HandleFunc("GET /stats/{code}", statsHandler)
	mux.HandleFunc("GET /recent", recentHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Shorten a URL
	body, _ := json.Marshal(ShortenRequest{URL: "https://example.com"})
	resp, err := http.Post(server.URL+"/shorten", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to post shorten: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var shortResp ShortenResponse
	if err := json.NewDecoder(resp.Body).Decode(&shortResp); err != nil {
		t.Fatalf("failed to decode shorten response: %v", err)
	}
	resp.Body.Close()

	if len(shortResp.Code) != 6 {
		t.Fatalf("expected 6-char code, got %q", shortResp.Code)
	}

	// Redirect
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err = client.Get(server.URL + "/" + shortResp.Code)
	if err != nil {
		t.Fatalf("failed to get redirect: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	loc, err := resp.Location()
	if err != nil {
		t.Fatalf("no location header: %v", err)
	}
	if loc.String() != "https://example.com" {
		t.Fatalf("expected redirect to https://example.com, got %s", loc.String())
	}
	resp.Body.Close()

	// Stats
	resp, err = http.Get(server.URL + "/stats/" + shortResp.Code)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var stats StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode stats: %v", err)
	}
	resp.Body.Close()

	if stats.URL != "https://example.com" {
		t.Fatalf("expected URL https://example.com, got %s", stats.URL)
	}
	if stats.Hits != 1 {
		t.Fatalf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}

	// Recent
	resp, err = http.Get(server.URL + "/recent")
	if err != nil {
		t.Fatalf("failed to get recent: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var recent []URLRecord
	if err := json.NewDecoder(resp.Body).Decode(&recent); err != nil {
		t.Fatalf("failed to decode recent: %v", err)
	}
	resp.Body.Close()
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent record, got %d", len(recent))
	}
}

func TestShortenMissingURL(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /shorten", shortenHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	body, _ := json.Marshal(ShortenRequest{URL: ""})
	resp, err := http.Post(server.URL+"/shorten", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRedirectNotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{code}", redirectHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/abcdef")
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGenerateCode(t *testing.T) {
	code, err := generateCode()
	if err != nil {
		t.Fatalf("generateCode failed: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("expected length 6, got %d", len(code))
	}
	for _, c := range code {
		if !strings.ContainsRune(codeAlphabet, c) {
			t.Fatalf("invalid char %q in code", c)
		}
	}
}
