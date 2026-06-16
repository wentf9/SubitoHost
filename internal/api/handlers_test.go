package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/engine"
)

func setupTestEngine(t *testing.T) *engine.Engine {
	t.Helper()
	cfg := config.Default()
	return engine.New(cfg)
}

func TestGetStatus(t *testing.T) {
	e := setupTestEngine(t)
	srv := NewServer(e, "127.0.0.1:0")

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestSetlistGotoNoSetlist(t *testing.T) {
	e := setupTestEngine(t)
	srv := NewServer(e, "127.0.0.1:0")

	req := httptest.NewRequest("POST", "/api/v1/setlist/goto", strings.NewReader(`{"index": 0}`))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when no setlist loaded, got %d", w.Code)
	}
}

// Ensure json encoder is used (suppress unused import warning)
var _ = json.Marshal

func TestRecordStartNoSetlist(t *testing.T) {
	e := setupTestEngine(t)
	srv := NewServer(e, "127.0.0.1:0")

	req := httptest.NewRequest("POST", "/api/v1/record/start", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 without setlist, got %d", w.Code)
	}
}

func TestRecordStopNotRecording(t *testing.T) {
	e := setupTestEngine(t)
	srv := NewServer(e, "127.0.0.1:0")

	req := httptest.NewRequest("POST", "/api/v1/record/stop", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 when not recording, got %d", w.Code)
	}
}

func TestRecordStatusIdle(t *testing.T) {
	e := setupTestEngine(t)
	srv := NewServer(e, "127.0.0.1:0")

	req := httptest.NewRequest("GET", "/api/v1/record/status", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "idle" {
		t.Errorf("status = %q, want idle", resp["status"])
	}
}
