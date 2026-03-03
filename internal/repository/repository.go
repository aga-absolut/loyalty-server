package repository

import (
	"context"

	"github.com/aga-absolut/LoyaltyProgram/internal/model"
)

type Storage interface {
	Withdraw(ctx context.Context, userID int, withdrawnRequest model.WithdrawRequest) error
	UpdateOrderProgress(ctx context.Context, newStatus, order, user string, accrual, withdrawn float64) error
	Withdrawals(ctx context.Context, userID int) ([]model.WithdrawResponse, error)
	UserAuthentication(ctx context.Context, login, password string) (int, error)
	UserRegistration(ctx context.Context, login, password string) (int, error)
	GetListOrders(ctx context.Context, userID int) ([]model.ListOrders, error)
	GetBalance(ctx context.Context, userID int) (model.Balance, error)
	AddOrderID(ctx context.Context, userID int, number string) error
	AddAccrual(ctx context.Context, userID int, sum float64) error
	
}
