package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"AVTproject/models"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) PostgresRepository {
	return PostgresRepository{db: db}
}

func (r PostgresRepository) GetUserByUsername(
	ctx context.Context,
	username string,
) (models.User, error) {
	row := r.db.QueryRowContext(
		ctx,
		"SELECT id, username, password, coins FROM users WHERE username=$1",
		username,
	)
	var u models.User
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.Coins)
	if err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r PostgresRepository) GetUserByID(
	ctx context.Context,
	id int,
) (models.User, error) {
	row := r.db.QueryRowContext(
		ctx,
		"SELECT id, username, password, coins FROM users WHERE id=$1",
		id,
	)
	var u models.User
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.Coins)
	if err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r PostgresRepository) CreateUser(
	ctx context.Context,
	username, password string,
) (int, error) {
	var id int
	err := r.db.QueryRowContext(
		ctx,
		"INSERT INTO users (username, password, coins) VALUES ($1, $2, 1000) RETURNING id",
		username, password,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r PostgresRepository) UpdateUserCoins(
	ctx context.Context,
	id, delta int,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var coins int
	err = tx.QueryRowContext(
		ctx,
		"SELECT coins FROM users WHERE id=$1 FOR UPDATE",
		id,
	).Scan(&coins)
	if err != nil {
		return err
	}

	newCoins := coins + delta
	if newCoins < 0 {
		return errors.New("недостаточно монет")
	}

	_, err = tx.ExecContext(
		ctx,
		"UPDATE users SET coins=$1 WHERE id=$2",
		newCoins, id,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r PostgresRepository) AddTransaction(
	ctx context.Context,
	fromUserID, toUserID, amount int,
) error {
	_, err := r.db.ExecContext(
		ctx,
		"INSERT INTO transactions (from_user_id, to_user_id, amount, created_at) "+
			"VALUES ($1, $2, $3, $4)",
		fromUserID, toUserID, amount, time.Now(),
	)
	return err
}

func (r PostgresRepository) GetUserTransactions(
	ctx context.Context,
	userID int,
) ([]models.Transaction, []models.Transaction, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, from_user_id, to_user_id, amount, created_at 
		 FROM transactions 
		 WHERE to_user_id=$1 
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var received []models.Transaction
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(
			&t.ID,
			&t.FromUserID,
			&t.ToUserID,
			&t.Amount,
			&t.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		received = append(received, t)
	}

	rows2, err := r.db.QueryContext(
		ctx,
		`SELECT id, from_user_id, to_user_id, amount, created_at 
		 FROM transactions 
		 WHERE from_user_id=$1 
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows2.Close()

	var sent []models.Transaction
	for rows2.Next() {
		var t models.Transaction
		if err := rows2.Scan(
			&t.ID,
			&t.FromUserID,
			&t.ToUserID,
			&t.Amount,
			&t.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		sent = append(sent, t)
	}
	return received, sent, nil
}

func (r PostgresRepository) GetUserPurchases(
	ctx context.Context,
	userID int,
) ([]models.Purchase, error) {
	rows, err := r.db.QueryContext(
		ctx,
		"SELECT id, user_id, item, quantity, created_at FROM purchases WHERE user_id=$1",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var purchases []models.Purchase
	for rows.Next() {
		var p models.Purchase
		if err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Item,
			&p.Quantity,
			&p.CreatedAt,
		); err != nil {
			return nil, err
		}
		purchases = append(purchases, p)
	}
	return purchases, nil
}

func (r PostgresRepository) AddPurchase(
	ctx context.Context,
	userID int,
	item string,
) error {
	var quantity int
	err := r.db.QueryRowContext(
		ctx,
		"SELECT quantity FROM purchases WHERE user_id=$1 AND item=$2",
		userID, item,
	).Scan(&quantity)
	if err != nil {
		if err == sql.ErrNoRows {
			_, err = r.db.ExecContext(
				ctx,
				"INSERT INTO purchases (user_id, item, quantity, created_at) VALUES ($1, $2, 1, $3)",
				userID, item, time.Now(),
			)
			return err
		}
		return err
	}
	_, err = r.db.ExecContext(
		ctx,
		"UPDATE purchases SET quantity = quantity + 1 WHERE user_id=$1 AND item=$2",
		userID, item,
	)
	return err
}
