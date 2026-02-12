package main

import (
	"gnss-service/internal/storage"

	"github.com/go-chi/chi"
	"go.uber.org/zap"
)

// main -  точка входа в приложения.
// Инициализирует роутер chi, создает хранилище отчетов и настраивает маршруты:
// - POST /api/user/register | Регистрация пользователя |
// - POST /api/user/login | Аутентификация пользователя |
// - POST /api/user/observation | Загрузка файла ГНСС наблюдений для обработки |
// - GET /api/user/history | Получение списка отчётов по обработке файлов пользователя |
// - GET /api/user/result/<номер_отчёта> | Получение отчёта с результатами обработки по указанному номеру |
// - GET /api/user/last | Получение последнего отчёта с результатами обработки |
// Запускает HTTP-сервер на порту 8080
// Также задает глобальные обработчики для MethodNotAllowed и NotFound.
func main() {
	parseFlags()

	logger, err := zap.NewDevelopment()
	if err != nil {
		logger.Fatal("cannot initialize zap")
	}
	defer logger.Sync()
	sugar := logger.Sugar()

	router := chi.NewRouter()

	if flagSQL != "" {
		sugar.Infof("Initializing PostrgeSQL storage with DSN: %s", flagSQL)

		dbStorage, err := storage.NewDBStorage(flagSQL)
		if err != nil {
			sugar.Fatalf("Fataled to save metrics on exit: %v", err)
		}
		defer func() {
			if err := dbStorage.Close(); err != nil {
				sugar.Errorf("Failed to close DB connection: %v", err)
			}
		}()
	}

}
