package middlewareservice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"gnss-service/internal/storage"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type (
	responseData struct {
		status int
		size   int
	}

	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}
)

type contextKey string

const UserLoginKey = contextKey("user_login")

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

func LogMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			responseData := &responseData{
				status: 0,
				size:   0,
			}

			lw := loggingResponseWriter{
				ResponseWriter: w,
				responseData:   responseData,
			}

			defer func() {
				if err := recover(); err != nil {
					logger.Errorf("PANIC recovered: %v", err)
					http.Error(&lw, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(&lw, r)
			duration := time.Since(start)
			logger.Infof("%s %s %d %v %d", r.RequestURI, r.Method, responseData.status, duration, responseData.size)
		})
	}
}

func AuthMiddleware(dbStorage *storage.DBStorage, logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "basic" {
				sendJSONError(w, "Invalid authorization format", http.StatusUnauthorized, logger)
				return
			}

			decoded, err := base64.StdEncoding.DecodeString(parts[1])

			if err != nil {
				sendJSONError(w, "Invalid credentials", http.StatusUnauthorized, logger)
				return
			}

			creds := strings.SplitN(string(decoded), ":", 2)
			if len(creds) != 2 {
				sendJSONError(w, "Invalid credentials format", http.StatusUnauthorized, logger)
				return
			}

			login, password := creds[0], creds[1]

			user, err := dbStorage.GetUser(login)
			if err != nil || user.Password != password {
				sendJSONError(w, "Invalid credentials", http.StatusUnauthorized, logger)
				return
			}

			err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
			if err != nil {
				logger.Warnf("Invalid password for user: %s", login)
				sendJSONError(w, "Unauthorized: invalid credentials", http.StatusUnauthorized, logger)
				return
			}

			// Добавляем логин в контекст
			ctx := context.WithValue(r.Context(), UserLoginKey, login)
			h.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserLogin(ctx context.Context) (string, bool) {
	login, ok := ctx.Value(UserLoginKey).(string)
	return login, ok
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func sendJSONResponse(w http.ResponseWriter, status int, data interface{}, logger *zap.SugaredLogger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Errorf("Failed to encode response: %v", err)
	}
}

func sendJSONError(w http.ResponseWriter, msg string, status int, logger *zap.SugaredLogger) {
	sendJSONResponse(w, status, ErrorResponse{Error: msg}, logger)
}
