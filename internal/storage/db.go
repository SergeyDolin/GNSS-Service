package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type User struct {
	Login    string
	Password string
}

type DBStorage struct {
	pool *pgxpool.Pool
}

type ObsFile struct {
	ID         int64     `json:"id"`
	UserLogin  string    `json:"user_login"`
	Filename   string    `json:"filename"`
	FileSize   int64     `json:"file_size"`
	UploadedAt time.Time `json:"uploaded_at"`
	Status     string    `json:"status"` // pending, processing, completed, failed
	ResultID   *int64    `json:"result_id,omitempty"`
}

type AdjustmentResult struct {
	ID        int64     `json:"id"`
	FileID    int64     `json:"file_id"`
	UserLogin string    `json:"user_login"`
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Z         float64   `json:"z"`
	SDX       float32   `json:"sdx"`
	SDY       float32   `json:"sdy"`
	SDZ       float32   `json:"sdz"`
	CreatedAt time.Time `json:"created_at"`
}

func NewDBStorage(dsn string) (*DBStorage, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to DB: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping DB: %w", err)
	}

	s := &DBStorage{
		pool: pool,
	}

	if err := s.initSchema(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return s, nil
}

func (s *DBStorage) initSchema() error {
	_, err := s.pool.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS users (
		login VARCHAR(255) PRIMARY KEY,
		password VARCHAR(255) NOT NULL
		);
 	`)
	if err != nil {
		return fmt.Errorf("Create users table: %w", err)
	}

	_, err = s.pool.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS observation_files (
		id SERIAL PRIMARY KEY,
		user_login VARCHAR(255) NOT NULL REFERENCES users(login) ON DELETE CASCADE,
		filename VARCHAR(500) NOT NULL,
		file_size BIGINT NOT NULL,
		uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		status VARCHAR(50) DEFAULT 'pending',
		result_id INTEGER
	);
	
	CREATE INDEX IF NOT EXISTS idx_files_user ON observation_files(user_login);
	CREATE INDEX IF NOT EXISTS idx_files_status ON observation_files(status);`)
	if err != nil {
		return fmt.Errorf("create files table: %w", err)
	}

	_, err = s.pool.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS adjustment_results (
		id SERIAL PRIMARY KEY,
		file_id INTEGER NOT NULL REFERENCES observation_files(id) ON DELETE CASCADE,
		user_login VARCHAR(255) NOT NULL REFERENCES users(login) ON DELETE CASCADE,
		x FLOAT8 NOT NULL,
		y FLOAT8 NOT NULL,
		z FLOAT8 NOT NULL,
		sdx FLOAT4,
		sdy FLOAT4,
		sdz FLOAT4,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_results_user ON adjustment_results(user_login);
	CREATE INDEX IF NOT EXISTS idx_results_file ON adjustment_results(file_id);`)
	if err != nil {
		return fmt.Errorf("create results table: %w", err)
	}

	return nil
}

func (s *DBStorage) CreateUser(login, password string) error {
	query := `INSERT INTO users (login, password) VALUES ($1, $2) ON CONFLICT (login) DO NOTHING`
	_, err := s.pool.Exec(context.Background(), query, login, password)
	if err != nil {
		return fmt.Errorf("create user %s: %w", login, err)
	}

	return nil
}

func (s *DBStorage) GetUser(login string) (*User, error) {
	query := `SELECT login, password FROM users WHERE login = $1`
	var user User
	err := s.pool.QueryRow(context.Background(), query, login).Scan(&user.Login, &user.Password)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user %s not found", login)
		}
		return nil, fmt.Errorf("get user %s: %w", login, err)
	}
	return &user, nil
}

func (s *DBStorage) UserExists(login string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE login = $1)`
	var exists bool
	err := s.pool.QueryRow(context.Background(), query, login).Scan(&exists)
	return exists, err
}

func (s *DBStorage) CreateFile(userLogin, filename string, fileSize int64) (*ObsFile, error) {
	query := `
		INSERT INTO observation_files (user_login, filename, file_size, uploaded_at, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, uploaded_at
	`

	var file ObsFile
	file.UserLogin = userLogin
	file.Filename = filename
	file.FileSize = fileSize
	file.Status = "pending"

	err := s.pool.QueryRow(
		context.Background(),
		query,
		file.UserLogin,
		file.Filename,
		file.FileSize,
		time.Now(),
		file.Status,
	).Scan(&file.ID, &file.UploadedAt)

	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}

	return &file, nil
}

func (s *DBStorage) GetFile(fileID int64) (*ObsFile, error) {
	query := `
		SELECT id, user_login, filename, file_size, uploaded_at, status, result_id
		FROM observation_files
		WHERE id = $1
	`

	var file ObsFile
	err := s.pool.QueryRow(context.Background(), query, fileID).Scan(
		&file.ID,
		&file.UserLogin,
		&file.Filename,
		&file.FileSize,
		&file.UploadedAt,
		&file.Status,
		&file.ResultID,
	)

	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}

	return &file, nil
}

func (s *DBStorage) GetUserFiles(userLogin string) ([]*ObsFile, error) {
	query := `
		SELECT id, user_login, filename, file_size, uploaded_at, status, result_id
		FROM observation_files
		WHERE user_login = $1
		ORDER BY uploaded_at DESC
	`

	rows, err := s.pool.Query(context.Background(), query, userLogin)
	if err != nil {
		return nil, fmt.Errorf("get user files: %w", err)
	}
	defer rows.Close()

	var files []*ObsFile
	for rows.Next() {
		var f ObsFile
		err := rows.Scan(
			&f.ID,
			&f.UserLogin,
			&f.Filename,
			&f.FileSize,
			&f.UploadedAt,
			&f.Status,
			&f.ResultID,
		)
		if err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}
		files = append(files, &f)
	}

	return files, nil
}

func (s *DBStorage) UpdateFileStatus(fileID int64, status string, resultID *int64) error {
	query := `
		UPDATE observation_files
		SET status = $2, result_id = $3
		WHERE id = $1
	`

	_, err := s.pool.Exec(context.Background(), query, fileID, status, resultID)
	if err != nil {
		return fmt.Errorf("update file status: %w", err)
	}

	return nil
}

func (s *DBStorage) SaveResult(result *AdjustmentResult) (int64, error) {
	query := `
		INSERT INTO adjustment_results (file_id, user_login, x, y, z, sdx, sdy, sdz, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	var id int64
	err := s.pool.QueryRow(
		context.Background(),
		query,
		result.FileID,
		result.UserLogin,
		result.X,
		result.Y,
		result.Z,
		result.SDX,
		result.SDY,
		result.SDZ,
		time.Now(),
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("save result: %w", err)
	}

	return id, nil
}

func (s *DBStorage) GetResult(resultID int64) (*AdjustmentResult, error) {
	query := `
		SELECT id, file_id, user_login, x, y, z, sdx, sdy, sdz, created_at
		FROM adjustment_results
		WHERE id = $1
	`

	var r AdjustmentResult
	err := s.pool.QueryRow(context.Background(), query, resultID).Scan(
		&r.ID,
		&r.FileID,
		&r.UserLogin,
		&r.X,
		&r.Y,
		&r.Z,
		&r.SDX,
		&r.SDY,
		&r.SDZ,
		&r.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("get result: %w", err)
	}

	return &r, nil
}

func (s *DBStorage) GetUserResults(userLogin string) ([]*AdjustmentResult, error) {
	query := `
		SELECT id, file_id, user_login, x, y, z, sdx, sdy, sdz, created_at
		FROM adjustment_results
		WHERE user_login = $1
		ORDER BY created_at DESC
	`

	rows, err := s.pool.Query(context.Background(), query, userLogin)
	if err != nil {
		return nil, fmt.Errorf("get user results: %w", err)
	}
	defer rows.Close()

	var results []*AdjustmentResult
	for rows.Next() {
		var r AdjustmentResult
		err := rows.Scan(
			&r.ID,
			&r.FileID,
			&r.UserLogin,
			&r.X,
			&r.Y,
			&r.Z,
			&r.SDX,
			&r.SDY,
			&r.SDZ,
			&r.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		results = append(results, &r)
	}

	return results, nil
}

func (s *DBStorage) GetLastUserResult(userLogin string) (*AdjustmentResult, error) {
	query := `
		SELECT id, file_id, user_login, x, y, z, sdx, sdy, sdz, created_at
		FROM adjustment_results
		WHERE user_login = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var r AdjustmentResult
	err := s.pool.QueryRow(context.Background(), query, userLogin).Scan(
		&r.ID,
		&r.FileID,
		&r.UserLogin,
		&r.X,
		&r.Y,
		&r.Z,
		&r.SDX,
		&r.SDY,
		&r.SDZ,
		&r.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("get last result: %w", err)
	}

	return &r, nil
}

func (s *DBStorage) Close() error {
	s.pool.Close()
	return nil
}
