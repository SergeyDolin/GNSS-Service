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
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")

			auth := r.Header.Get("Authorization")
			if auth == "" {
				logger.Warn("AuthMiddleware: missing Authorization header")
				w.Header().Set("WWW-Authenticate", "Basic realm=\"GNSS Service\"")
				sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
				return
			}

			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "basic" {
				logger.Warnf("AuthMiddleware: invalid Authorization format")
				w.Header().Set("WWW-Authenticate", "Basic realm=\"GNSS Service\"")
				sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
				return
			}

			decoded, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				logger.Warnf("AuthMiddleware: failed to decode base64")
				w.Header().Set("WWW-Authenticate", "Basic realm=\"GNSS Service\"")
				sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
				return
			}

			credentials := strings.SplitN(string(decoded), ":", 2)
			if len(credentials) != 2 {
				logger.Warnf("AuthMiddleware: invalid credentials format")
				w.Header().Set("WWW-Authenticate", "Basic realm=\"GNSS Service\"")
				sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
				return
			}

			login, password := credentials[0], credentials[1]

			user, err := dbStorage.GetUser(login)
			if err != nil {
				logger.Warnf("AuthMiddleware: user not found: %s", login)
				w.Header().Set("WWW-Authenticate", "Basic realm=\"GNSS Service\"")
				sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
				return
			}

			err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
			if err != nil {
				logger.Warnf("AuthMiddleware: password mismatch for user: %s", login)
				w.Header().Set("WWW-Authenticate", "Basic realm=\"GNSS Service\"")
				sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
				return
			}

			logger.Infof("AuthMiddleware: authentication successful for user: %s", login)

			ctx := context.WithValue(r.Context(), UserLoginKey, login)
			next.ServeHTTP(w, r.WithContext(ctx))
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
