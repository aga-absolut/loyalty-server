package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aga-absolut/LoyaltyProgram/internal/app"
	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/internal/router"
	"github.com/aga-absolut/LoyaltyProgram/internal/storage"
	"github.com/aga-absolut/LoyaltyProgram/internal/storage/database"
	"github.com/aga-absolut/LoyaltyProgram/internal/workers"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	processChan := make(chan string, 10)
	cfg := config.NewConfig()
	logger := logger.NewLogger()
	storage := storage.NewStorage(cfg, logger)
	worker := workers.NewPollWorker(ctx, processChan, storage, config.SizeWorkers, logger, cfg)
	app := app.NewApp(cfg, logger, storage, processChan)
	router := router.NewRouter(app)
	if err := database.InitMigrations(cfg, logger); err != nil {
		logger.Fatalw("error to init migrations", "error", err)
	}

	server := &http.Server{
		Addr:    cfg.RunAddress,
		Handler: router,
	}
	go func() {
		logger.Infow("Starting server", "addr", cfg.RunAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorw("Server error", "Error", err)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutdown signal received")

	ShutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ShutdownCtx); err != nil {
		logger.Errorw("Server shutdown error", "Error", err)
	}

	worker.Stop()
	logger.Info("Application stopped successfully")
}
