package handlers

import (
	"encoding/json"
	"net/http"

	"gnss-service/internal/storage"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Message string `json:"message"`
	Login   string `json:"login"`
}

func RegisterHandler(dbStorage *storage.DBStorage, logger *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendJSONError(w, "Only POST allowed", http.StatusMethodNotAllowed, logger)
			return
		}

		var req AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSONError(w, "Invalid request", http.StatusBadRequest, logger)
			return
		}

		exists, err := dbStorage.UserExists(req.Login)
		if err != nil {
			logger.Errorf("Failed to check user: %v", err)
			sendJSONError(w, "Internal error", http.StatusInternalServerError, logger)
			return
		}
		if exists {
			sendJSONError(w, "User already exists", http.StatusConflict, logger)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			logger.Errorf("Failed to hash password: %v", err)
			sendJSONError(w, "Internal error", http.StatusInternalServerError, logger)
			return
		}

		if err := dbStorage.CreateUser(req.Login, string(hashedPassword)); err != nil {
			logger.Errorf("Failed to create user: %v", err)
			sendJSONError(w, "Failed to create user", http.StatusInternalServerError, logger)
			return
		}

		sendJSONResponse(w, http.StatusCreated, AuthResponse{
			Message: "User registered successfully",
			Login:   req.Login,
		}, logger)
	}
}

func LoginHandler(dbStorage *storage.DBStorage, logger *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendJSONError(w, "Only POST allowed", http.StatusMethodNotAllowed, logger)
			return
		}

		var req AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSONError(w, "Invalid request", http.StatusBadRequest, logger)
			return
		}

		user, err := dbStorage.GetUser(req.Login)
		if err != nil {
			logger.Warnf("Login failed for user %s: %v", req.Login, err)
			sendJSONError(w, "Invalid credentials", http.StatusUnauthorized, logger)
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
		if err != nil {
			logger.Warnf("Invalid password for user %s", req.Login)
			sendJSONError(w, "Invalid credentials", http.StatusUnauthorized, logger)
			return
		}

		sendJSONResponse(w, http.StatusOK, AuthResponse{
			Message: "Successfully authenticated",
			Login:   req.Login,
		}, logger)
	}
}

func LogoutHandler(logger *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendJSONError(w, "Only POST allowed", http.StatusMethodNotAllowed, logger)
			return
		}
		sendJSONResponse(w, http.StatusOK, map[string]string{
			"message": "Logged out successfully",
		}, logger)
	}
}
