package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gnss-service/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestAuth(t *testing.T) (*storage.DBStorage, *zap.SugaredLogger) {
	logger, _ := zap.NewDevelopment()
	sugar := logger.Sugar()

	dsn := "postgres://postgres:1337@localhost:5432/gnssservice_test?sslmode=disable"
	db, err := storage.NewDBStorage(dsn)
	require.NoError(t, err)

	// Очищаем таблицу users
	db.GetUser("cleanup") // just to ensure connection works

	return db, sugar
}

func TestRegisterHandler_Success(t *testing.T) {
	db, sugar := setupTestAuth(t)
	defer db.Close()

	reqBody := AuthRequest{
		Login:    "newuser",
		Password: "secret123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler := RegisterHandler(db, sugar)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp AuthResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.Equal(t, "User registered successfully", resp.Message)
	assert.Equal(t, "newuser", resp.Login)
}

func TestRegisterHandler_UserExists(t *testing.T) {
	db, sugar := setupTestAuth(t)
	defer db.Close()

	// Создаем пользователя
	db.CreateUser("existing", "hash")

	reqBody := AuthRequest{
		Login:    "existing",
		Password: "secret",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler := RegisterHandler(db, sugar)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestLoginHandler_Success(t *testing.T) {
	db, sugar := setupTestAuth(t)
	defer db.Close()

	// Регистрируем пользователя
	handler := RegisterHandler(db, sugar)
	regBody := AuthRequest{Login: "loginuser", Password: "pass123"}
	regBodyJSON, _ := json.Marshal(regBody)
	regReq := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(regBodyJSON))
	regReq.Header.Set("Content-Type", "application/json")
	regW := httptest.NewRecorder()
	handler.ServeHTTP(regW, regReq)

	// Логинимся
	loginBody := AuthRequest{Login: "loginuser", Password: "pass123"}
	body, _ := json.Marshal(loginBody)

	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	loginHandler := LoginHandler(db, sugar)
	loginHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp AuthResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	assert.NoError(t, err)
	assert.Equal(t, "Successfully authenticated", resp.Message)
	assert.Equal(t, "loginuser", resp.Login)
}

func TestLoginHandler_WrongPassword(t *testing.T) {
	db, sugar := setupTestAuth(t)
	defer db.Close()

	db.CreateUser("testuser", "$2a$10$hashedpassword")

	reqBody := AuthRequest{
		Login:    "testuser",
		Password: "wrongpass",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler := LoginHandler(db, sugar)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
