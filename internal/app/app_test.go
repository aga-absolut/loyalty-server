package app

import (
	"testing"

	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
)

func TestApp_RegisterHandler(t *testing.T) {
	cfg := &config.Config{}
	log := &logger.Logger{}
	ch := make(chan string)
	app := NewApp(cfg, log, testStorage, ch)
	type Credentials struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	tests := []struct {
		name        string
		statusCode  int
		contentType string
		Credentials Credentials
	}{
		{
			name:        "first test",
			statusCode:  200,
			contentType: "application/json",
			Credentials: Credentials{
				Login:    "absolute",
				Password: "123456",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}
