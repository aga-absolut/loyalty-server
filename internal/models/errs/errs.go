package errs

import "errors"

var (
	ErrIncorrectLogin       = errors.New("incorrect login or password")
	ErrLoginAlreadyUsed     = errors.New("login already used")
	ErrNoUserID             = errors.New("unauthorized")
	ErrInvalidOrderID       = errors.New("order id isn`t valid")
	ErrOrderIDUsed          = errors.New("order used")
	ErrOrderIDUsedByAnother = errors.New("order used")
	ErrNotEnoughMoney       = errors.New("not enough money")
	ErrTooManyRequests      = errors.New("429 Too Many Requests")
)
