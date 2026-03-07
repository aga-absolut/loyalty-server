package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/internal/models"
	"github.com/aga-absolut/LoyaltyProgram/internal/models/errs"
	"github.com/aga-absolut/LoyaltyProgram/internal/storage/database"
	"github.com/aga-absolut/LoyaltyProgram/internal/tools"
	"github.com/aga-absolut/LoyaltyProgram/middleware/auth"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
)

type Storage interface {
	Withdraw(ctx context.Context, userID int, withdrawnRequest models.WithdrawRequest) error
	UpdateOrderStatus(ctx context.Context, orderID, status string, accrual float64) error
	Withdrawals(ctx context.Context, userID int) ([]models.WithdrawResponse, error)
	UserAuthentication(ctx context.Context, login, password string) (int, error)
	UserRegistration(ctx context.Context, login, password string) (int, error)
	GetListOrders(ctx context.Context, userID int) ([]models.ListOrders, error)
	GetBalance(ctx context.Context, userID int) (models.Balance, error)
	AddOrderID(ctx context.Context, userID int, number string) error
}

type App struct {
	config      *config.Config
	logger      *logger.Logger
	storage     Storage
	processChan chan string
}

func NewApp(config *config.Config, logger *logger.Logger, storage Storage, processChan chan string) *App {
	return &App{
		config:      config,
		logger:      logger,
		storage:     storage,
		processChan: processChan,
	}
}

func NewStorage(config *config.Config, logger *logger.Logger) Storage {
	logger.Infow("connect to Postgres")
	return database.NewDatabase(config, logger)
}

func (a *App) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var credential models.Credentials
	if err := json.NewDecoder(r.Body).Decode(&credential); err != nil {
		http.Error(w, "error in decoding the request body", http.StatusBadRequest)
		return
	}

	if credential.Login == "" || credential.Password == "" {
		http.Error(w, "login or password is empty", http.StatusBadRequest)
		return
	}

	hashPassword := tools.HashSha256(credential.Password)
	userID, err := a.storage.UserRegistration(r.Context(), credential.Login, hashPassword)
	if err != nil {
		if errors.Is(err, errs.ErrLoginAlreadyUsed) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		a.logger.Errorw("error in user registration", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token, err := auth.BuildJWTString(userID)
	if err != nil {
		a.logger.Errorw("error to create JWT", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	cookie := auth.BuildCookie(token)
	http.SetCookie(w, cookie)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (a *App) AuthHandler(w http.ResponseWriter, r *http.Request) {
	var credential models.Credentials
	if err := json.NewDecoder(r.Body).Decode(&credential); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if credential.Login == "" || credential.Password == "" {
		http.Error(w, "login or password is empty", http.StatusBadRequest)
		return
	}

	hashPassword := tools.HashSha256(credential.Password)
	userID, err := a.storage.UserAuthentication(r.Context(), credential.Login, hashPassword)
	if err != nil {
		if errors.Is(err, errs.ErrIncorrectLogin) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		a.logger.Errorw("error in user authentication", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token, err := auth.BuildJWTString(userID)
	if err != nil {
		a.logger.Errorw("error to create JWT", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	cookie := auth.BuildCookie(token)
	http.SetCookie(w, cookie)
	w.WriteHeader(http.StatusOK)
}

func (a *App) AddOrderIDHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if errors.Is(err, errs.ErrNoUserID) {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	orderID := string(data)
	if ok := tools.CheckOrderID(orderID); !ok {
		http.Error(w, errs.ErrInvalidOrderID.Error(), http.StatusUnprocessableEntity)
		return
	}

	err = a.storage.AddOrderID(r.Context(), userID, orderID)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrOrderIDUsed):
			w.WriteHeader(http.StatusOK)
		case errors.Is(err, errs.ErrOrderIDUsedByAnother):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			a.logger.Errorw("internal server error", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	a.processChan <- orderID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
}

func (a *App) GetListOrdersHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	data, err := a.storage.GetListOrders(r.Context(), userID)
	if err != nil {
		a.logger.Errorw("error to get list orders", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(data) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		a.logger.Errorw("failed encode data", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (a *App) GetBalanceHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	data, err := a.storage.GetBalance(r.Context(), userID)
	if err != nil {
		a.logger.Errorw("error to get balance", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		a.logger.Errorw("failed encode data", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (a *App) WithdrawHandler(w http.ResponseWriter, r *http.Request) {
	var withdrawRequest models.WithdrawRequest
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&withdrawRequest); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if withdrawRequest.Sum <= 0 {
		http.Error(w, "not valid sum", http.StatusBadRequest)
		return
	}

	if ok := tools.CheckOrderID(withdrawRequest.Order); !ok {
		http.Error(w, errs.ErrInvalidOrderID.Error(), http.StatusPaymentRequired)
		return
	}

	if err := a.storage.Withdraw(r.Context(), userID, withdrawRequest); err != nil {
		if errors.Is(err, errs.ErrNotEnoughMoney) {
			http.Error(w, err.Error(), http.StatusPaymentRequired)
			return
		}
		a.logger.Errorw("error to withdraw", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (a *App) GetWithdrawalsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	data, err := a.storage.Withdrawals(r.Context(), userID)
	if err != nil {
		a.logger.Errorw("failed get withdrawls", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(data) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		a.logger.Errorw("failed encode data", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
