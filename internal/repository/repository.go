package repository

import (
	"context"

	"github.com/aga-absolut/LoyaltyProgram/internal/models"
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