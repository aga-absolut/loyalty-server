package config

import (
	"flag"
	"time"

	"github.com/caarlos0/env/v11"
)

var (
	TokenExpTime = 10 * time.Minute
	SecretKey    = []byte("absolute_secret_key")
	SizeWorkers  = 1
)

type Config struct {
	RunAddress    string `env:"RUN_ADDRESS"`
	DatabaseDSN   string `env:"DATABASE_URI"`
	SystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

func NewConfig() *Config {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil
	}
	flag.StringVar(&cfg.RunAddress, "a", "", "address and port")                             // localhost:8080
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "name to connect database")                    // postgres://postgres:absolute_1@localhost:5432/LoyalProgram
	flag.StringVar(&cfg.SystemAddress, "r", "", "address of the accrual calculation system") // http://localhost:35067
	flag.Parse()
	return cfg
}
