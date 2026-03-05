package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/internal/models"
	"github.com/aga-absolut/LoyaltyProgram/internal/models/errs"
	"github.com/aga-absolut/LoyaltyProgram/internal/repository"
	"github.com/aga-absolut/LoyaltyProgram/middleware/auth"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
)

type App struct {
	config      *config.Config
	logger      *logger.Logger
	storage     repository.Storage
	processChan chan string
}

func NewApp(config *config.Config, logger *logger.Logger, storage repository.Storage, processChan chan string) *App {
	return &App{
		config:      config,
		logger:      logger,
		storage:     storage,
		processChan: processChan,
	}
}

func (a *App) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	credential := models.Credentials{}
	if err := json.NewDecoder(r.Body).Decode(&credential); err != nil {
		http.Error(w, "error in decoding the request body", http.StatusBadRequest)
		return
	}

	if credential.Login == "" || credential.Password == "" {
		http.Error(w, "login or password is empty", http.StatusBadRequest)
		return
	}

	userID, err := a.storage.UserRegistration(r.Context(), credential.Login, credential.Password)
	if err != nil {
		if errors.Is(err, errs.ErrLoginAlreadyUsed) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "error in user registration", http.StatusInternalServerError)
		return
	}

	token, err := auth.BuildJWTString(userID)
	if err != nil {
		http.Error(w, "error to create JWT", http.StatusInternalServerError)
		return
	}
	cookie := auth.BuildCookie(token)
	http.SetCookie(w, cookie)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (a *App) AuthHandler(w http.ResponseWriter, r *http.Request) {
	credential := models.Credentials{}
	if err := json.NewDecoder(r.Body).Decode(&credential); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if credential.Login == "" || credential.Password == "" {
		http.Error(w, "login or password is empty", http.StatusConflict)
		return
	}

	userID, err := a.storage.UserAuthentication(r.Context(), credential.Login, credential.Password)
	if err != nil {
		if errors.Is(err, errs.ErrIncorrectLogin) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Error(w, "error in user authentication", http.StatusInternalServerError)
		return
	}

	token, err := auth.BuildJWTString(userID)
	if err != nil {
		http.Error(w, "error to create JWT", http.StatusInternalServerError)
		return
	}
	cookie := auth.BuildCookie(token)
	http.SetCookie(w, cookie)
	w.WriteHeader(http.StatusOK)
}

func (a *App) AddOrderIDHandler(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	orderID := string(data)

	userID, err := auth.GetUserIDFromContext(r.Context())
	if errors.Is(err, errs.ErrNoUserID) {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	err = a.storage.AddOrderID(r.Context(), userID, orderID)
	if err != nil {
		switch {
		case errors.Is(err, errs.ErrInvalidOrderID):
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		case errors.Is(err, errs.ErrOrderIDUsed):
			w.WriteHeader(http.StatusOK)
		case errors.Is(err, errs.ErrOrderIDUsedByAnother):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
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
		http.Error(w, "error to get list orders", http.StatusInternalServerError)
		return
	}

	if len(data) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, "error to get balance", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) WithdrawHandler(w http.ResponseWriter, r *http.Request) {
	withdrawRequest := models.WithdrawRequest{}
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

	if err := a.storage.Withdraw(r.Context(), userID, withdrawRequest); err != nil {
		if errors.Is(err, errs.ErrNotEnoughMoney) {
			http.Error(w, err.Error(), http.StatusPaymentRequired)
			return
		}
		if errors.Is(err, errs.ErrInvalidOrderID) {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (a *App) GetWithdraawalsHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	data, err := a.storage.Withdrawals(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(data) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// addition
func (a *App) AddAccrualHandler(w http.ResponseWriter, r *http.Request) {
	sum := models.Sum{}
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&sum); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if sum.Sum <= 0 {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := a.storage.AddAccrual(r.Context(), userID, sum.Sum); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
}
