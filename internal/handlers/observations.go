package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gnss-service/cmd/rtklib/processor"
	middlewareservice "gnss-service/internal/middleware-service"
	"gnss-service/internal/storage"

	"go.uber.org/zap"
)

const maxFileSize = 1024 * 1024 * 1000

type ErrorResponse struct {
	Error string `json:"error"`
}

type UploadResponse struct {
	Message    string `json:"message"`
	FileID     int64  `json:"file_id"`
	Filename   string `json:"filename"`
	FileSize   int64  `json:"file_size"`
	UploadedAt string `json:"uploaded_at"`
	Status     string `json:"status"`
}

type FileStatusResponse struct {
	ID         int64     `json:"id"`
	Filename   string    `json:"filename"`
	FileSize   int64     `json:"file_size"`
	UploadedAt time.Time `json:"uploaded_at"`
	Status     string    `json:"status"`
	ResultID   *int64    `json:"result_id,omitempty"`
}

func UploadFileHandler(
	dbStorage *storage.DBStorage,
	processor *processor.RINEXProcessor,
	logger *zap.SugaredLogger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			sendJSONError(w, "Only POST allowed", http.StatusMethodNotAllowed, logger)
			return
		}

		userLogin, ok := middlewareservice.GetUserLogin(r.Context())
		if !ok {
			sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)

		if err := r.ParseMultipartForm(maxFileSize); err != nil {
			sendJSONError(w, "File too large", http.StatusRequestEntityTooLarge, logger)
			return
		}
		defer r.MultipartForm.RemoveAll()

		file, header, err := r.FormFile("rinex_file")
		if err != nil {
			sendJSONError(w, "No file uploaded", http.StatusBadRequest, logger)
			return
		}
		defer file.Close()

		if !isRINEXFile(header.Filename) {
			sendJSONError(w, "Invalid file format", http.StatusBadRequest, logger)
			return
		}

		fileData, err := io.ReadAll(file)
		if err != nil {
			sendJSONError(w, "Failed to read file", http.StatusInternalServerError, logger)
			return
		}

		obsFile, err := dbStorage.CreateFile(userLogin, header.Filename, int64(len(fileData)))
		if err != nil {
			logger.Errorf("Failed to create file record: %v", err)
			sendJSONError(w, "Failed to save file info", http.StatusInternalServerError, logger)
			return
		}

		go processor.ProcessFile(obsFile.ID, userLogin, fileData, dbStorage, logger)

		response := UploadResponse{
			Message:    "File uploaded and queued for processing",
			FileID:     obsFile.ID,
			Filename:   header.Filename,
			FileSize:   obsFile.FileSize,
			UploadedAt: obsFile.UploadedAt.Format(time.RFC3339),
			Status:     "pending",
		}

		sendJSONResponse(w, http.StatusAccepted, response, logger)
	}
}

func GetFileStatusHandler(
	dbStorage *storage.DBStorage,
	logger *zap.SugaredLogger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			sendJSONError(w, "Only GET allowed", http.StatusMethodNotAllowed, logger)
			return
		}

		fileIDStr := strings.TrimPrefix(r.URL.Path, "/api/user/file/")
		fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
		if err != nil {
			sendJSONError(w, "Invalid file ID", http.StatusBadRequest, logger)
			return
		}

		userLogin, ok := middlewareservice.GetUserLogin(r.Context())
		if !ok {
			sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
			return
		}

		file, err := dbStorage.GetFile(fileID)
		if err != nil {
			sendJSONError(w, "File not found", http.StatusNotFound, logger)
			return
		}

		if file.UserLogin != userLogin {
			sendJSONError(w, "Access denied", http.StatusForbidden, logger)
			return
		}

		response := FileStatusResponse{
			ID:         file.ID,
			Filename:   file.Filename,
			FileSize:   file.FileSize,
			UploadedAt: file.UploadedAt,
			Status:     file.Status,
			ResultID:   file.ResultID,
		}

		sendJSONResponse(w, http.StatusOK, response, logger)
	}
}

func GetResultHandler(
	dbStorage *storage.DBStorage,
	logger *zap.SugaredLogger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			sendJSONError(w, "Only GET allowed", http.StatusMethodNotAllowed, logger)
			return
		}

		resultIDStr := strings.TrimPrefix(r.URL.Path, "/api/user/result/")
		resultID, err := strconv.ParseInt(resultIDStr, 10, 64)
		if err != nil {
			sendJSONError(w, "Invalid result ID", http.StatusBadRequest, logger)
			return
		}

		userLogin, ok := middlewareservice.GetUserLogin(r.Context())
		if !ok {
			sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
			return
		}

		result, err := dbStorage.GetResult(resultID)
		if err != nil {
			sendJSONError(w, "Result not found", http.StatusNotFound, logger)
			return
		}

		if result.UserLogin != userLogin {
			sendJSONError(w, "Access denied", http.StatusForbidden, logger)
			return
		}

		sendJSONResponse(w, http.StatusOK, result, logger)
	}
}

func GetLastResultHandler(
	dbStorage *storage.DBStorage,
	logger *zap.SugaredLogger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			sendJSONError(w, "Only GET allowed", http.StatusMethodNotAllowed, logger)
			return
		}

		userLogin, ok := middlewareservice.GetUserLogin(r.Context())
		if !ok {
			sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
			return
		}

		result, err := dbStorage.GetLastUserResult(userLogin)
		if err != nil {
			sendJSONError(w, "No results found", http.StatusNotFound, logger)
			return
		}

		sendJSONResponse(w, http.StatusOK, result, logger)
	}
}

func GetHistoryHandler(
	dbStorage *storage.DBStorage,
	logger *zap.SugaredLogger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			sendJSONError(w, "Only GET allowed", http.StatusMethodNotAllowed, logger)
			return
		}

		userLogin, ok := middlewareservice.GetUserLogin(r.Context())
		if !ok {
			sendJSONError(w, "Unauthorized", http.StatusUnauthorized, logger)
			return
		}

		files, err := dbStorage.GetUserFiles(userLogin)
		if err != nil {
			logger.Errorf("Failed to get user files: %v", err)
			sendJSONError(w, "Failed to get history", http.StatusInternalServerError, logger)
			return
		}

		sendJSONResponse(w, http.StatusOK, files, logger)
	}
}

func isRINEXFile(filename string) bool {
	exts := []string{".rnx", ".obs", ".nav", ".RINEX"}
	filename = strings.ToLower(filename)
	for _, ext := range exts {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
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
