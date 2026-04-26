package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID         string
	Username   string
	Password   string
	NGACNodeID string
}

type Store struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) CreateUser(ctx context.Context, id, username, password, ngacNodeID string) error {
	_, err := s.db.Exec(ctx,
		"INSERT INTO users (id, username, password, ngac_node) VALUES ($1, $2, $3, $4)",
		id, username, password, ngacNodeID)
	return err
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := s.db.QueryRow(ctx,
		"SELECT id, username, password, ngac_node FROM users WHERE username = $1",
		username).Scan(&u.ID, &u.Username, &u.Password, &u.NGACNodeID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

func (s *Store) GetUserByID(ctx context.Context, userID string) (*User, error) {
	var u User
	err := s.db.QueryRow(ctx,
		"SELECT id, username, password, ngac_node FROM users WHERE id = $1",
		userID).Scan(&u.ID, &u.Username, &u.Password, &u.NGACNodeID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

func (s *Store) GetUserByNGACNodeID(ctx context.Context, ngacNodeID string) (*User, error) {
	var u User
	err := s.db.QueryRow(ctx,
		"SELECT id, username, password, ngac_node FROM users WHERE ngac_node = $1",
		ngacNodeID).Scan(&u.ID, &u.Username, &u.Password, &u.NGACNodeID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user by ngac node: %w", err)
	}
	return &u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.Query(ctx, "SELECT id, username, ngac_node FROM users ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.NGACNodeID); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}
