package storage

import (
	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/internal/repository"
	"github.com/aga-absolut/LoyaltyProgram/internal/storage/database"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
)

func NewStorage(config *config.Config, logger *logger.Logger) repository.Storage {
	logger.Infow("connect to Postgres")
	return database.NewDatabase(config, logger)
}
