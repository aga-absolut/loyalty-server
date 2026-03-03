-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS withdrawals CASCADE;

CREATE TABLE users(
    id SERIAL PRIMARY KEY,
    user_login TEXT NOT NULL UNIQUE,
    user_password TEXT NOT NULL,
    user_balance DECIMAL(10,2) DEFAULT 0.0,
    total_withdrawn DECIMAL(10,2) DEFAULT 0.0,
    uploaded_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE orders(
    user_id INTEGER NOT NULL,
    order_id TEXT NOT NULL UNIQUE,
    order_status TEXT NOT NULL DEFAULT 'NEW',
    accrual DECIMAL(10,2) DEFAULT 0.0,
    uploaded_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE withdrawals(
    user_id INTEGER NOT NULL,
    order_id TEXT NOT NULL UNIQUE,
    amount DECIMAL(10,2),
    processed_at TIMESTAMPTZ DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd