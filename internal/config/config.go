package config

import (
	"flag"
	"time"

	"github.com/caarlos0/env/v11"
)

var (
	TokenExpTime = 10 * time.Minute
	SecretKey    = []byte("absolute_secret_key")
	SizeWorkers  = 10
)

type Config struct {
	RunAddress    string `env:"RUN_ADDRESS"`
	DatabaseURI   string `env:"DATABASE_URI"`
	SystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

func NewConfig() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.RunAddress, "a", "localhost:8080", "address and port")
	flag.StringVar(&cfg.DatabaseURI, "d", "postgres://postgres:absolute_1@localhost:5432/LoyalProgram", "name to connect database")
	flag.StringVar(&cfg.SystemAddress, "r", "", "")
	if err := env.Parse(cfg); err != nil {
		return nil
	}
	return cfg
}
