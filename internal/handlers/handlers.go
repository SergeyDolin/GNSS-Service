package handlers

import (
	"encoding/json"
	"gnss-service/internal/storage"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	Message string `json:"message"`
}

func IndexHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(res, "Only GET request allowed!", http.StatusMethodNotAllowed)
		return
	}

	res.Header().Set("Content-Type", "text/html; charset=utf-8")

	html := `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Metrics</title></head>
<body><h1>GNSS Service (Collaborative positioning)</h1></body></html>`
	io.WriteString(res, html)
}

func RegisterHandler(dbStorage *storage.DBStorage, logger *zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST requests allowed", http.StatusMethodNotAllowed)
			return
		}

		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Errorf("Failed to decode register request: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if req.Login == "" || req.Password == "" {
			http.Error(w, "Login and password are required", http.StatusBadRequest)
			return
		}

		if _, exists := dbStorage.GetUser(req.Login); exists {
			http.Error(w, "User already exists", http.StatusConflict)
			return
		}

		if err := dbStorage.CreateUser(req.Login, req.Password); err != nil {
			logger.Errorf("Failed to create user %s: %v", req.Login, err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		response := RegisterResponse{
			Message: "User successfully registered",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.Errorf("Failed to encode response: %v", err)
		}

		logger.Infof("New user registered: %s", req.Login)
	}
}
