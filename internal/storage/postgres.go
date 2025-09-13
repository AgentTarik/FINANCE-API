package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStore struct {
	DB *sql.DB
}

func NewPostgres(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(1 * time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return &PostgresStore{DB: db}, nil
}

// Users Repo

func (p *PostgresStore) CreateUser(u User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := p.DB.ExecContext(ctx,
		`INSERT INTO users (id, name) VALUES ($1, $2)`, u.ID, u.Name)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return ErrUserAlreadyExists
		}
		return err
	}
	return nil
}

func (p *PostgresStore) GetUser(id uuid.UUID) (User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var u User
	err := p.DB.QueryRowContext(ctx,
		`SELECT id, name FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	return u, nil
}

// Transactions Repo

func (p *PostgresStore) UpsertTx(t Transaction) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := p.DB.ExecContext(ctx, `
		INSERT INTO transactions (transaction_id, user_id, amount, timestamp, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (transaction_id) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    amount  = EXCLUDED.amount,
		    timestamp = EXCLUDED.timestamp,
		    status  = EXCLUDED.status
	`, t.TransactionID, t.UserID, t.Amount, t.Timestamp, t.Status)
	return err
}

func (p *PostgresStore) ListTx() ([]Transaction, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := p.DB.QueryContext(ctx, `
		SELECT transaction_id, user_id, amount, timestamp, status
		FROM transactions
		ORDER BY timestamp DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.TransactionID, &t.UserID, &t.Amount, &t.Timestamp, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
