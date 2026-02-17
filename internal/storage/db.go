package storage

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type AdjustmentResult struct {
	id  int
	x   float64
	y   float64
	z   float64
	sdx float32
	sdy float32
	sdz float32
}

type DBStorage struct {
	conn  *pgx.Conn
	users map[string]*MemStorageUser
	mu    sync.RWMutex
}

func NewDBStorage(dsn string) (*DBStorage, error) {
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to DB: %w", err)
	}

	s := &DBStorage{
		conn:  conn,
		users: make(map[string]*MemStorageUser),
	}

	if err := s.initSchema(); err != nil {
		conn.Close(context.Background())
		return nil, fmt.Errorf("init schema: %w", err)
	}

	if err := s.loadFromDB(); err != nil {
		conn.Close(context.Background())
		return nil, fmt.Errorf("load from DB: %w", err)
	}

	return s, nil
}

func (s *DBStorage) initSchema() error {
	_, err := s.conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS users (
		login VARCHAR(255) PRIMARY KEY,
		password VARCHAR(255) NOT NULL
		);
 	`)
	if err != nil {
		return fmt.Errorf("Create users table: %w", err)
	}

	return nil
}

func (s *DBStorage) loadFromDB() error {
	rows, err := s.conn.Query(context.Background(), `SELECT login, password FROM users`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var login, password string

		if err := rows.Scan(&login, &password); err != nil {
			return fmt.Errorf("scan user: %w", err)
		}

		user := &MemStorageUser{
			login:       login,
			password:    password,
			adjustments: *NewMemStorage(),
		}

		s.users[login] = user
	}
	return nil
}

func (s *DBStorage) CreateUser(login, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO users (login, password) VALUES ($1, $2) ON CONFLICT (login) DO NOTHING`
	_, err := s.conn.Exec(context.Background(), query, login, password)
	if err != nil {
		return fmt.Errorf("create user %s: %w", login, err)
	}

	user := &MemStorageUser{
		login:       login,
		password:    password,
		adjustments: *NewMemStorage(),
	}

	s.users[login] = user
	return nil
}

func (s *DBStorage) GetUser(login string) (*MemStorageUser, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[login]
	if exists {
		return user, true
	}

	var password string

	err := s.conn.QueryRow(context.Background(),
		`SELECT password FROM users WHERE login = $1`, login).Scan(&password)
	if err != nil {
		return nil, false
	}

	user = &MemStorageUser{
		login:       login,
		password:    password,
		adjustments: *NewMemStorage(),
	}

	s.users[login] = user
	return user, true
}

func (s *DBStorage) UpdateUserPassword(login, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `UPDATE users SET password = $2 WHERE login = $1`
	_, err := s.conn.Exec(context.Background(), query, login, newPassword)
	if err != nil {
		return fmt.Errorf("update user password %s: %w", login, err)
	}

	if user, exists := s.users[login]; exists {
		user.password = newPassword
	}

	return nil
}

func (s *DBStorage) Close() error {
	return s.conn.Close(context.Background())
}
