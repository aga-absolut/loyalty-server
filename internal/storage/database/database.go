package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"runtime"
	"strconv"
	"unicode"

	"github.com/aga-absolut/LoyaltyProgram/internal/config"
	"github.com/aga-absolut/LoyaltyProgram/internal/errs"
	"github.com/aga-absolut/LoyaltyProgram/internal/model"
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
	db, err := sql.Open("pgx", config.DatabaseURI)
	if err != nil {
		return nil
	}

	return &Database{
		config: config,
		logger: logger,
		db:     db,
	}
}

func (d *Database) UserRegistration(ctx context.Context, login, password string) (int, error) {
	var userID int
	hashPassword := HashSha256(password)
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

func (d *Database) UserAuthentication(ctx context.Context, login, password string) (int, error) {
	var userID int
	hashPassword := HashSha256(password)
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
	if ok := CheckOrderID(orderID); !ok {
		return errs.ErrInvalidOrderID
	}
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

func (d *Database) GetListOrders(ctx context.Context, userID int) ([]model.ListOrders, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT order_id, order_status, accrual, uploaded_at FROM orders
	WHERE user_id = $1 ORDER BY uploaded_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	orders := make([]model.ListOrders, 0)
	for rows.Next() {
		order := model.ListOrders{}
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

func (d *Database) GetBalance(ctx context.Context, userID int) (model.Balance, error) {
	balance := model.Balance{}
	err := d.db.QueryRowContext(ctx, `SELECT user_balance, total_withdrawn FROM users
	WHERE id = $1`, userID).Scan(&balance.Current, &balance.WithDrawn)
	if err != nil {
		return balance, err
	}
	return balance, nil
}

func (d *Database) Withdraw(ctx context.Context, userID int, withdrawnRequest model.WithdrawRequest) error {
	var balance float64
	if ok := CheckOrderID(withdrawnRequest.Order); !ok {
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

	if balance >= withdrawnRequest.Sum {
		balance -= withdrawnRequest.Sum
	} else {
		return errs.ErrNotEnoughMoney
	}

	_, err = tx.ExecContext(ctx, `UPDATE users SET user_balance = $1 WHERE id = $2`, balance, userID)
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

func (d *Database) Withdrawals(ctx context.Context, userID int) ([]model.WithdrawResponse, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT order_id, amount, processed_at FROM withdrawals 
	WHERE user_id = $1 ORDER BY processed_at DESC`, userID)
	if err != nil {
		return nil, err
	}

	withdrawals := make([]model.WithdrawResponse, 0)
	for rows.Next() {
		withdrawal := model.WithdrawResponse{}
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
    err = tx.QueryRowContext(ctx,`SELECT user_id FROM orders WHERE order_id = $1`,orderID,).Scan(&userID)
    if err != nil {
        return err
    }

    _, err = tx.ExecContext(ctx,`UPDATE orders SET order_status = $1, accrual = $2 
	WHERE order_id = $3`,status, accrual, orderID,)
    if err != nil {
        return err
    }

    if status == "PROCESSED" && accrual > 0 {
        _, err = tx.ExecContext(ctx,`UPDATE users SET user_balance = user_balance + $1 
		WHERE id = $2`,accrual, userID,)
        if err != nil {
            return err
        }
    }

    return tx.Commit()
}

// addition
func (d *Database) AddAccrual(ctx context.Context, userID int, sum float64) error {
	_, err := d.db.ExecContext(ctx, `UPDATE users SET user_balance = user_balance + $1 WHERE id = $2`, sum, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errs.ErrNoUserID
		}
		return err
	}
	return nil
}

func InitMigrations(config *config.Config, logger *logger.Logger) error {
	logger.Infow("Starting migrations", "database", config.DatabaseURI)

	db, err := sql.Open("pgx", config.DatabaseURI)
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

func CheckOrderID(orderID string) bool {
	for _, r := range orderID {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	sum := 0
	for i := len(orderID) - 1; i >= 0; i-- {
		digit, _ := strconv.Atoi(string(orderID[i]))
		if (len(orderID)-i)%2 == 0 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}

func HashSha256(password string) string {
	data := sha256.Sum256([]byte(password))
	return hex.EncodeToString(data[:])
}
