package storage

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDBName = "gnssservice_test"
	testDSN    = "postgres://postgres:1337@localhost:5432/" + testDBName + "?sslmode=disable"
)

func setupTestDB(t *testing.T) *DBStorage {
	ensureTestDBExists(t)

	db, err := NewDBStorage(testDSN)
	require.NoError(t, err)

	cleanupTables(t, db)

	return db
}

func ensureTestDBExists(t *testing.T) {
	adminDSN := "postgres://postgres:1337@localhost:5432/postgres?sslmode=disable"

	adminPool, err := pgxpool.New(context.Background(), adminDSN)
	require.NoError(t, err)
	defer adminPool.Close()

	var exists bool
	err = adminPool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", testDBName).Scan(&exists)
	require.NoError(t, err)

	if !exists {
		_, err = adminPool.Exec(context.Background(), "CREATE DATABASE "+testDBName)
		require.NoError(t, err)
	}
}

func cleanupTables(t *testing.T, db *DBStorage) {
	ctx := context.Background()
	tables := []string{"adjustment_results", "observation_files", "users"}

	for _, table := range tables {
		_, err := db.pool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE")
		require.NoError(t, err)
	}
}

func createTestUser(t *testing.T, db *DBStorage, login, password string) {
	err := db.CreateUser(login, password)
	require.NoError(t, err)
}

func createTestFile(t *testing.T, db *DBStorage, userLogin, filename string, size int64) *ObsFile {
	file, err := db.CreateFile(userLogin, filename, size)
	require.NoError(t, err)
	return file
}

func createTestResult(t *testing.T, db *DBStorage, fileID int64, userLogin string, x, y, z float64) int64 {
	result := &AdjustmentResult{
		FileID:    fileID,
		UserLogin: userLogin,
		X:         x,
		Y:         y,
		Z:         z,
		SDX:       0.1,
		SDY:       0.2,
		SDZ:       0.3,
	}
	id, err := db.SaveResult(result)
	require.NoError(t, err)
	return id
}

func int64Ptr(i int64) *int64 {
	return &i
}

func TestNewDBStorage(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		ensureTestDBExists(t)
		db, err := NewDBStorage(testDSN)
		assert.NoError(t, err)
		assert.NotNil(t, db)
		db.Close()
	})

	t.Run("invalid DSN", func(t *testing.T) {
		db, err := NewDBStorage("invalid-dsn")
		assert.Error(t, err)
		assert.Nil(t, db)
	})
}

func TestCreateAndGetUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := db.CreateUser("testuser", "password123")
	assert.NoError(t, err)

	user, err := db.GetUser("testuser")
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "testuser", user.Login)
	assert.Equal(t, "password123", user.Password)
}

func TestCreateDuplicateUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := db.CreateUser("testuser", "password123")
	assert.NoError(t, err)

	err = db.CreateUser("testuser", "newpassword")
	assert.NoError(t, err)

	user, err := db.GetUser("testuser")
	assert.NoError(t, err)
	assert.Equal(t, "password123", user.Password)
}

func TestGetUserNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	user, err := db.GetUser("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, user)
	assert.Contains(t, err.Error(), "not found")
}

func TestUserExists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")

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

	createTestUser(t, db, "testuser", "password123")

	file, err := db.CreateFile("testuser", "test.obs", 1024)
	assert.NoError(t, err)
	assert.NotNil(t, file)
	assert.NotZero(t, file.ID)
	assert.Equal(t, "testuser", file.UserLogin)
	assert.Equal(t, "test.obs", file.Filename)
	assert.Equal(t, int64(1024), file.FileSize)
	assert.Equal(t, "pending", file.Status)
	assert.Nil(t, file.ResultID)
}

func TestCreateFileUserNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	file, err := db.CreateFile("nonexistent", "test.obs", 1024)
	assert.Error(t, err)
	assert.Nil(t, file)
}

func TestGetFile(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")
	created := createTestFile(t, db, "testuser", "test.obs", 1024)

	file, err := db.GetFile(created.ID)
	assert.NoError(t, err)
	assert.NotNil(t, file)
	assert.Equal(t, created.ID, file.ID)
	assert.Equal(t, "testuser", file.UserLogin)
	assert.Equal(t, "test.obs", file.Filename)
	assert.Equal(t, int64(1024), file.FileSize)
}

func TestGetFileNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	file, err := db.GetFile(99999)
	assert.Error(t, err)
	assert.Nil(t, file)
}

func TestGetUserFilesEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")

	files, err := db.GetUserFiles("testuser")
	assert.NoError(t, err)
	assert.Empty(t, files)
}

func TestUpdateFileStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")
	file := createTestFile(t, db, "testuser", "test.obs", 1024)

	testCases := []struct {
		name     string
		status   string
		resultID *int64
	}{
		{"processing", "processing", nil},
		{"completed with result", "completed", int64Ptr(123)},
		{"failed", "failed", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := db.UpdateFileStatus(file.ID, tc.status, tc.resultID)
			assert.NoError(t, err)

			updated, err := db.GetFile(file.ID)
			assert.NoError(t, err)
			assert.Equal(t, tc.status, updated.Status)

			if tc.resultID != nil {
				assert.NotNil(t, updated.ResultID)
				assert.Equal(t, *tc.resultID, *updated.ResultID)
			} else {
				assert.Nil(t, updated.ResultID)
			}
		})
	}
}

func TestSaveAndGetResult(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")
	file := createTestFile(t, db, "testuser", "test.obs", 1024)

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
	assert.NotNil(t, saved)
	assert.Equal(t, id, saved.ID)
	assert.Equal(t, file.ID, saved.FileID)
	assert.Equal(t, "testuser", saved.UserLogin)
	assert.Equal(t, 123.456, saved.X)
	assert.Equal(t, 789.012, saved.Y)
	assert.Equal(t, 345.678, saved.Z)
	assert.Equal(t, float32(0.123), saved.SDX)
	assert.Equal(t, float32(0.456), saved.SDY)
	assert.Equal(t, float32(0.789), saved.SDZ)
}

func TestGetResultNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	result, err := db.GetResult(99999)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetUserResults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")
	file1 := createTestFile(t, db, "testuser", "file1.obs", 1024)
	file2 := createTestFile(t, db, "testuser", "file2.obs", 1024)

	createTestResult(t, db, file1.ID, "testuser", 1, 2, 3)
	time.Sleep(10 * time.Millisecond)
	createTestResult(t, db, file2.ID, "testuser", 4, 5, 6)

	results, err := db.GetUserResults("testuser")
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	if len(results) == 2 {
		assert.True(t, results[0].CreatedAt.After(results[1].CreatedAt) ||
			results[0].CreatedAt.Equal(results[1].CreatedAt))
	}
}

func TestGetUserResultsEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")

	results, err := db.GetUserResults("testuser")
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestGetLastUserResult(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")
	file := createTestFile(t, db, "testuser", "test.obs", 1024)

	createTestResult(t, db, file.ID, "testuser", 1, 2, 3)
	time.Sleep(10 * time.Millisecond)
	createTestResult(t, db, file.ID, "testuser", 4, 5, 6)
	time.Sleep(10 * time.Millisecond)
	lastID := createTestResult(t, db, file.ID, "testuser", 7, 8, 9)

	last, err := db.GetLastUserResult("testuser")
	assert.NoError(t, err)
	assert.NotNil(t, last)
	assert.Equal(t, lastID, last.ID)
	assert.Equal(t, 7.0, last.X)
	assert.Equal(t, 8.0, last.Y)
	assert.Equal(t, 9.0, last.Z)
}

func TestGetLastUserResultNoResults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")

	last, err := db.GetLastUserResult("testuser")
	assert.Error(t, err)
	assert.Nil(t, last)
}

func TestCascadeDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")
	file := createTestFile(t, db, "testuser", "test.obs", 1024)
	resultID := createTestResult(t, db, file.ID, "testuser", 1, 2, 3)

	user, err := db.GetUser("testuser")
	assert.NoError(t, err)
	assert.NotNil(t, user)

	ctx := context.Background()
	_, err = db.pool.Exec(ctx, "DELETE FROM users WHERE login = $1", "testuser")
	assert.NoError(t, err)

	_, err = db.GetUser("testuser")
	assert.Error(t, err)

	_, err = db.GetFile(file.ID)
	assert.Error(t, err)

	_, err = db.GetResult(resultID)
	assert.Error(t, err)
}

func TestConcurrentOperations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	createTestUser(t, db, "testuser", "password123")

	done := make(chan bool)
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(n int) {
			file, err := db.CreateFile("testuser", "test.obs", 1024)
			if err != nil {
				errChan <- err
				done <- true
				return
			}

			_, err = db.SaveResult(&AdjustmentResult{
				FileID:    file.ID,
				UserLogin: "testuser",
				X:         float64(n),
				Y:         float64(n),
				Z:         float64(n),
				SDX:       0.1,
				SDY:       0.1,
				SDZ:       0.1,
			})
			if err != nil {
				errChan <- err
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	close(errChan)
	for err := range errChan {
		assert.NoError(t, err)
	}

	files, err := db.GetUserFiles("testuser")
	assert.NoError(t, err)
	assert.Len(t, files, 10)

	results, err := db.GetUserResults("testuser")
	assert.NoError(t, err)
	assert.Len(t, results, 10)
}

func TestInitSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	ensureTestDBExists(t)
	db, err := NewDBStorage(testDSN)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	tables := []string{"users", "observation_files", "adjustment_results"}

	for _, table := range tables {
		var exists bool
		err = db.pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)",
			table).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "Table %s should exist", table)
	}
}
