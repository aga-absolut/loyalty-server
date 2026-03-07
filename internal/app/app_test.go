package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
	"github.com/stretchr/testify/assert"
)

func TestApp_RegisterHandler(t *testing.T) {
	cfg := &config.Config{DatabaseDSN: "postgres://postgres:absolute_1@localhost:5432/LoyalProgram"}
	log := logger.NewLogger()
	storage := NewStorage(cfg, log)
	ch := make(chan string)
	app := NewApp(cfg, log, storage, ch)
	tests := []struct {
		name        string
		statusCode  int
		contentType string
		body        string
	}{
		{"first test", http.StatusOK, "application/json", `{"login":"absolute1", "password":"12345678"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			app.RegisterHandler(w, r)
			res := w.Result()

			assert.Equal(t, tt.contentType, res.Header.Get("Content-Type"))
			assert.Equal(t, tt.statusCode, res.StatusCode)
		})
	}
}

func TestApp_AuthHandler(t *testing.T) {
	cfg := &config.Config{DatabaseDSN: "postgres://postgres:absolute_1@localhost:5432/LoyalProgram"}
	log := logger.NewLogger()
	storage := NewStorage(cfg, log)
	ch := make(chan string)
	app := NewApp(cfg, log, storage, ch)

	tests := []struct {
		name        string
		statusCode  int
		contentType string
		body        string
	}{
		{"first test", http.StatusUnauthorized, "text/plain; charset=utf-8", `{"login":"absolute123", "password":"12345678"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			app.AuthHandler(w, r)
			res := w.Result()

			assert.Equal(t, tt.contentType, res.Header.Get("Content-Type"))
			assert.Equal(t, tt.statusCode, res.StatusCode)
		})
	}
}
