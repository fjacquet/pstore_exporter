package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fjacquet/pstore_exporter/internal/powerstore"
)

func TestLivezReturnsOK(t *testing.T) {
	rec := httptest.NewRecorder()
	staticOKHandler(rec, httptest.NewRequest(http.MethodGet, "/livez", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReadyzReturnsOK(t *testing.T) {
	rec := httptest.NewRecorder()
	staticOKHandler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHealthReturns200WhenAllArraysDown(t *testing.T) {
	store := powerstore.NewSnapshotStore()
	store.Store(powerstore.BuildSnapshot([]*powerstore.ArraySnapshot{
		{Array: "pstore-01", Up: false, ScrapeError: "login POST: status 401", LastScrape: time.Now()},
	}))
	server := &Server{store: store}

	rec := httptest.NewRecorder()
	server.healthHandler(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body struct {
		Arrays []struct {
			Array string `json:"array"`
			OK    bool   `json:"ok"`
			Err   string `json:"err"`
		} `json:"arrays"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(body.Arrays) != 1 || body.Arrays[0].OK {
		t.Fatalf("arrays = %+v, want one array with ok=false", body.Arrays)
	}
	if body.Arrays[0].Err == "" {
		t.Fatalf("err field empty, want the scrape failure message")
	}
}

func TestHealthReturns200BeforeFirstCycle(t *testing.T) {
	server := &Server{store: powerstore.NewSnapshotStore()}

	rec := httptest.NewRecorder()
	server.healthHandler(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
