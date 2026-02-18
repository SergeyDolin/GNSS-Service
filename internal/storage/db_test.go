package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain - подготовка тестовой БД
func TestMain(m *testing.M) {
	// Создаем тестовую БД, если её нет
	conn, err := pgx.Connect(context.Background(), "postgres://postgres:1337@localhost:5432/postgres?sslmode=disable")
	if err == nil {
		conn.Exec(context.Background(), "CREATE DATABASE gnssservice_test")
		conn.Close(context.Background())
	}

	os.Exit(m.Run())
}

func setupTestDB(t *testing.T) *DBStorage {
	dsn := "postgres://postgres:1337@localhost:5432/gnssservice_test?sslmode=disable"

	// Очищаем таблицы
	conn, err := pgx.Connect(context.Background(), dsn)
	require.NoError(t, err)
	defer conn.Close(context.Background())

	conn.Exec(context.Background(), "TRUNCATE TABLE users CASCADE")

	db, err := NewDBStorage(dsn)
	require.NoError(t, err)

	return db
}

func TestCreateAndGetUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Создание пользователя
	err := db.CreateUser("testuser", "hashedpassword")
	assert.NoError(t, err)

	// Получение пользователя
	user, err := db.GetUser("testuser")
	assert.NoError(t, err)
	assert.Equal(t, "testuser", user.Login)
	assert.Equal(t, "hashedpassword", user.Password)
}

func TestUserExists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("testuser", "pass")

	exists, err := db.UserExists("testuser")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = db.UserExists("nonexistent")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestCreateFile(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("testuser", "pass")

	file, err := db.CreateFile("testuser", "test.obs", 1024)
	assert.NoError(t, err)
	assert.NotZero(t, file.ID)
	assert.Equal(t, "testuser", file.UserLogin)
	assert.Equal(t, "test.obs", file.Filename)
	assert.Equal(t, int64(1024), file.FileSize)
	assert.Equal(t, "pending", file.Status)
}

func TestGetFile(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("testuser", "pass")
	created, _ := db.CreateFile("testuser", "test.obs", 1024)

	file, err := db.GetFile(created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.ID, file.ID)
	assert.Equal(t, "testuser", file.UserLogin)
}

func TestGetUserFiles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("testuser", "pass")
	db.CreateFile("testuser", "file1.obs", 1024)
	db.CreateFile("testuser", "file2.obs", 2048)

	files, err := db.GetUserFiles("testuser")
	assert.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestUpdateFileStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("testuser", "pass")
	file, _ := db.CreateFile("testuser", "test.obs", 1024)

	err := db.UpdateFileStatus(file.ID, "processing", nil)
	assert.NoError(t, err)

	updated, _ := db.GetFile(file.ID)
	assert.Equal(t, "processing", updated.Status)
}

func TestSaveAndGetResult(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("testuser", "pass")
	file, _ := db.CreateFile("testuser", "test.obs", 1024)

	result := &AdjustmentResult{
		FileID:    file.ID,
		UserLogin: "testuser",
		X:         123.456,
		Y:         789.012,
		Z:         345.678,
		SDX:       0.123,
		SDY:       0.456,
		SDZ:       0.789,
	}

	id, err := db.SaveResult(result)
	assert.NoError(t, err)
	assert.NotZero(t, id)

	saved, err := db.GetResult(id)
	assert.NoError(t, err)
	assert.Equal(t, 123.456, saved.X)
	assert.Equal(t, 789.012, saved.Y)
	assert.Equal(t, 345.678, saved.Z)
}

func TestGetUserResults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("testuser", "pass")
	file1, _ := db.CreateFile("testuser", "file1.obs", 1024)
	file2, _ := db.CreateFile("testuser", "file2.obs", 1024)

	db.SaveResult(&AdjustmentResult{FileID: file1.ID, UserLogin: "testuser", X: 1, Y: 2, Z: 3})
	db.SaveResult(&AdjustmentResult{FileID: file2.ID, UserLogin: "testuser", X: 4, Y: 5, Z: 6})

	results, err := db.GetUserResults("testuser")
	assert.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestGetLastUserResult(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	db.CreateUser("testuser", "pass")
	file, _ := db.CreateFile("testuser", "test.obs", 1024)

	db.SaveResult(&AdjustmentResult{FileID: file.ID, UserLogin: "testuser", X: 1, Y: 2, Z: 3})
	time.Sleep(1 * time.Second)
	db.SaveResult(&AdjustmentResult{FileID: file.ID, UserLogin: "testuser", X: 4, Y: 5, Z: 6})

	last, err := db.GetLastUserResult("testuser")
	assert.NoError(t, err)
	assert.Equal(t, 4.0, last.X)
}
