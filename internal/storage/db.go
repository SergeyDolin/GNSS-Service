package storage

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx"
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
		password VARCHAR(255) NOT NULL,
		results_id INT
		);
 	`)
	if err != nil {
		return fmt.Errorf("Create users table: %w", err)
	}

	_, err = s.conn.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS results (
			id SERIAL PRIMARY KEY,
			user_login VARCHAR(255) NOT NULL,
			x DOUBLE PRECISION NOT NULL,
			y DOUBLE PRECISION NOT NULL,
			z DOUBLE PRECISION NOT NULL,
			sdx FLOAT NOT NULL,
			sdy FLOAT NOT NULL,
			sdz FLOAT NOT NULL,
			FOREIGN KEY (user_login) REFERENCES user(login) ON DELETE CASCADE
			);`)
	if err != nil {
		return fmt.Errorf("Create results table: %w", err)
	}

	var exists bool
	err = s.conn.QueryRow(context.Background(), `
		SELECT EXISTS (
				SELECT 1
				FROM pg_constraint
				WHERE conname = 'fk_users_results');`).Scan(&exists)
	if err == nill && !exists {
		_, err = s.conn.Exec(context.Background(), `
		ALTER TABLE users
		ADD CONSTRAINT fk_users_results
		FOREIGN KEY (results_id) REFERENCES results(id);`)
		if err != nil {
			return nil
		}
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

func (s *DBStorage) SaveAdjustment(login string, x, y, z float64, sdx, sdy, sdz float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var exists bool

	err := s.conn.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM users WHERE login = $1)`, login).Scan(&exists)
	if err != nil {
		fmt.Errorf("check user existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("user %s not found", login)
	}
	_, err = s.conn.Exec(context.Background(), `
		INSERT INTO results (user_login, x, y, z, sdx, sdy, sdz) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, login, x, y, z, sdx, sdy, sdz)
	if err != nil {
		return fmt.Errorf("save adjustment for user %s: %w", login, err)
	}

	return nil
}
func (s *DBStorage) GetUserAdjustment(login string) (*AdjustmentResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result AdjustmentResult
	err := s.conn.QueryRow(context.Background(), `
		SELECT id, x, y, z, sdx, sdy, sdz 
		FROM results 
		WHERE user_login = $1 
		ORDER BY id DESC 
		LIMIT 1
	`, login).Scan(&result.id, &result.x, &result.y, &result.z, &result.sdx, &result.sdy, &result.sdz)

	if err != nil {
		return nil, fmt.Errorf("get adjustment for user %s: %w", login, err)
	}

	return &result, nil
}

func (s *DBStorage) GetAllUserAdjustments(login string) ([]AdjustmentResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.conn.Query(context.Background(), `
		SELECT id, x, y, z, sdx, sdy, sdz 
		FROM results 
		WHERE user_login = $1 
		ORDER BY id DESC
	`, login)
	if err != nil {
		return nil, fmt.Errorf("query adjustments for user %s: %w", login, err)
	}
	defer rows.Close()

	var results []AdjustmentResult
	for rows.Next() {
		var result AdjustmentResult
		if err := rows.Scan(&result.id, &result.x, &result.y, &result.z, &result.sdx, &result.sdy, &result.sdz); err != nil {
			return nil, fmt.Errorf("scan adjustment: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

func (s *DBStorage) SaveEstCoor(login, key string, value float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[login]
	if !exists {
		return fmt.Errorf("user %s not found", login)
	}

	user.adjustments.mu.Lock()
	user.adjustments.estCoor[key] = value
	user.adjustments.mu.Unlock()

	return nil
}

func (s *DBStorage) SaveRmsInter(login, key string, value float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[login]
	if !exists {
		return fmt.Errorf("user %s not found", login)
	}

	user.adjustments.mu.Lock()
	user.adjustments.rmsInter[key] = value
	user.adjustments.mu.Unlock()

	return nil
}

func (s *DBStorage) GetEstCoor(login, key string) (float32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[login]
	if !exists {
		return 0, false
	}

	user.adjustments.mu.RLock()
	defer user.adjustments.mu.RUnlock()

	val, exists := user.adjustments.estCoor[key]
	return val, exists
}

func (s *DBStorage) GetRmsInter(login, key string) (float32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[login]
	if !exists {
		return 0, false
	}

	user.adjustments.mu.RLock()
	defer user.adjustments.mu.RUnlock()

	val, exists := user.adjustments.rmsInter[key]
	return val, exists
}

func (s *DBStorage) GetAllUserAdjustmentsData(login string) (*MemStorageAdj, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[login]
	if !exists {
		return nil, fmt.Errorf("user %s not found", login)
	}

	return &user.adjustments, nil
}

func (s *DBStorage) Close() error {
	return s.conn.Close(context.Background())
}
