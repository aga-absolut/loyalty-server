package database

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"

	"github.com/aga-absolut/LoyaltyProgram/internal/app"
	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/internal/models"
	"github.com/aga-absolut/LoyaltyProgram/internal/models/errs"
	"github.com/aga-absolut/LoyaltyProgram/internal/tools"
	"github.com/aga-absolut/LoyaltyProgram/middleware/logger"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose"
)

type Database struct {
	config *config.Config
	logger *logger.Logger
	db     *sql.DB
}

func NewDatabase(config *config.Config, logger *logger.Logger) *Database {
	db, err := sql.Open("pgx", config.DatabaseDSN)
	if err != nil {
		return nil
	}

	return &Database{
		config: config,
		logger: logger,
		db:     db,
	}
}

func NewStorage(config *config.Config, logger *logger.Logger) app.Storage {
	logger.Infow("connect to Postgres")
	return NewDatabase(config, logger)
}

func (d *Database) UserRegistration(ctx context.Context, login, hashPassword string) (int, error) {
	var userID int
	err := d.db.QueryRowContext(ctx, `INSERT INTO users (user_login, user_password) 
	VALUES ($1, $2) RETURNING id`, login, hashPassword).Scan(&userID)
	if err != nil {
		var Pgerr *pgconn.PgError
		if errors.As(err, &Pgerr) && Pgerr.Code == pgerrcode.UniqueViolation {
			return 0, errs.ErrLoginAlreadyUsed
		}
		return 0, err
	}
	return userID, nil
}

func (d *Database) UserAuthentication(ctx context.Context, login, hashPassword string) (int, error) {
	var userID int
	row := d.db.QueryRowContext(ctx, `SELECT id FROM users 
	WHERE user_login = $1 AND user_password = $2`, login, hashPassword)
	if err := row.Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errs.ErrIncorrectLogin
		}
		return 0, err
	}
	return userID, nil
}

func (d *Database) AddOrderID(ctx context.Context, userID int, orderID string) error {
	var checkUserID int
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `SELECT user_id FROM orders WHERE order_id = $1`, orderID).Scan(&checkUserID)
	if err == nil {
		if checkUserID == userID {
			return errs.ErrOrderIDUsed
		}
		return errs.ErrOrderIDUsedByAnother
	} else {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO orders (user_id, order_id, order_status) 
	VALUES ($1, $2, 'NEW')`, userID, orderID)
	if err != nil {
		var Pgerr *pgconn.PgError
		if errors.As(err, &Pgerr) && Pgerr.Code == pgerrcode.UniqueViolation {
			return errs.ErrOrderIDUsed
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (d *Database) GetListOrders(ctx context.Context, userID int) ([]models.ListOrders, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT order_id, order_status, accrual, uploaded_at FROM orders
	WHERE user_id = $1 ORDER BY uploaded_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]models.ListOrders, 0)
	for rows.Next() {
		order := models.ListOrders{}
		err := rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}

func (d *Database) GetBalance(ctx context.Context, userID int) (models.Balance, error) {
	balance := models.Balance{}
	err := d.db.QueryRowContext(ctx, `SELECT user_balance, total_withdrawn FROM users
	WHERE id = $1`, userID).Scan(&balance.Current, &balance.WithDrawn)
	if err != nil {
		return balance, err
	}
	return balance, nil
}

func (d *Database) Withdraw(ctx context.Context, userID int, withdrawnRequest models.WithdrawRequest) error {
	var balance float64
	if ok := tools.CheckOrderID(withdrawnRequest.Order); !ok {
		return errs.ErrInvalidOrderID
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `SELECT user_balance FROM users 
	WHERE id = $1`, userID).Scan(&balance)
	if err != nil {
		return err
	}

	if balance < withdrawnRequest.Sum {
		return errs.ErrNotEnoughMoney
	}

	_, err = tx.ExecContext(ctx, `UPDATE users SET user_balance = user_balance - $1,total_withdrawn = total_withdrawn + $1 
    WHERE id = $2`, withdrawnRequest.Sum, userID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO withdrawals (user_id, order_id, amount, processed_at) 
	VALUES ($1, $2, $3, NOW())`, userID, withdrawnRequest.Order, withdrawnRequest.Sum)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (d *Database) Withdrawals(ctx context.Context, userID int) ([]models.WithdrawResponse, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT order_id, amount, processed_at FROM withdrawals 
	WHERE user_id = $1 ORDER BY processed_at DESC`, userID)
	if err != nil {
		return nil, err
	}

	withdrawals := make([]models.WithdrawResponse, 0)
	for rows.Next() {
		withdrawal := models.WithdrawResponse{}
		if err := rows.Scan(&withdrawal.Order, &withdrawal.Sum, &withdrawal.ProcessedAt); err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return withdrawals, nil
}

func (d *Database) UpdateOrderStatus(ctx context.Context, orderID, status string, accrual float64) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID int
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM orders WHERE order_id = $1`, orderID).Scan(&userID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `UPDATE orders SET order_status = $1, accrual = $2 
	WHERE order_id = $3`, status, accrual, orderID)
	if err != nil {
		return err
	}

	if status == "PROCESSED" && accrual > 0 {
		_, err = tx.ExecContext(ctx, `UPDATE users SET user_balance = user_balance + $1 
		WHERE id = $2`, accrual, userID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func InitMigrations(config *config.Config, logger *logger.Logger) error {
	logger.Infow("Starting migrations", "database", config.DatabaseDSN)

	db, err := sql.Open("pgx", config.DatabaseDSN)
	if err != nil {
		return err
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		return err
	}
	_, filename, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(filename), "..", "migrations")

	if err = goose.SetDialect("postgres"); err != nil {
		return err
	}
	err = goose.Up(db, migrationsPath)
	if err != nil {
		return err
	}
	return nil
}
