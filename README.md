# go-musthave-diploma-tpl

Репозиторий индивидуального дипломного проекта курса «Go-разработчик»

# GNSS Service

Сервис для обработки GNSS наблюдений (RINEX файлов) с использованием RTKLIB. Автоматически определяет дату наблюдений, скачивает соответствующие эфемериды с IGS сервера и выполняет постобработку для определения координат.

## 🚀 Возможности

- **Аутентификация и авторизация** - регистрация пользователей, Basic Auth для защищенных маршрутов
- **Загрузка RINEX файлов** - поддержка форматов .rnx, .obs, .nav
- **Автоматическое определение даты** - парсинг заголовка RINEX для определения времени наблюдений
- **Скачивание эфемерид** - автоматическая загрузка бортовых эфемерид с IGS сервера
- **Обработка через RTKLIB** - запуск rnx2rtkp с предустановленной конфигурацией
- **Сохранение результатов** - хранение координат и точностных характеристик в PostgreSQL
- **REST API** - полный набор эндпоинтов для доступа к файлам и результатам
- **Конкурентная обработка** - пул воркеров для параллельной обработки нескольких файлов
- **Автоматическая очистка** - временные файлы удаляются после обработки

## 📋 Требования

- **Go** 1.23 или выше
- **PostgreSQL** 14 или выше
- **RTKLIB** (rnx2rtkp) - включен в репозиторий
- **gunzip** - для распаковки эфемерид (обычно предустановлен)
- **Доступ в интернет** - для скачивания эфемерид с IGS сервера

## 🔧 Установка

### 1. Клонирование репозитория

```bash
git clone https://github.com/SergeyDolin/GNSS-Service.git
cd GNSS-Service
```

### 2. Установка зависимостей

```bash
go mod download
```

### 3. Настройка базы данных

Создайте базу данных в PostgreSQL:

```sql
CREATE DATABASE gnssservice;
CREATE USER gnssuser WITH PASSWORD 'your_password';
GRANT ALL PRIVILEGES ON DATABASE gnssservice TO gnssuser;
```

### 4. Проверка RTKLIB

Убедитесь, что RTKLIB собран и доступен:

```bash
chmod +x cmd/rtklib/app/rnx2rtkp
./cmd/rtklib/app/rnx2rtkp -h
```

## 🏃 Запуск

### Базовый запуск

```bash
go run cmd/gnss-service/main.go -d "postgres://gnssuser:your_password@localhost:5432/gnssservice?sslmode=disable"
```

### С указанием адреса

```bash
go run cmd/gnss-service/main.go -a "localhost:9090" -d "postgres://gnssuser:your_password@localhost:5432/gnssservice?sslmode=disable"
```

### Параметры командной строки

| Параметр | Описание | Значение по умолчанию |
|----------|----------|----------------------|
| `-a` | Адрес и порт сервера | `localhost:8080` |
| `-d` | DSN для подключения к PostgreSQL | `postgres://postgres:1337@localhost:5432/gnssservice?sslmode=disable` |

### Переменные окружения

| Переменная | Описание | Переопределяет |
|------------|----------|----------------|
| `ADDRESS` | Адрес сервера | `-a` |
| `DATABASE_DSN` | DSN для подключения к БД | `-d` |

## 📚 API Endpoints

### Публичные маршруты (без аутентификации)

#### Регистрация пользователя
```bash
POST /api/user/register
Content-Type: application/json

{
    "login": "username",
    "password": "password"
}
```

**Ответ:**
```json
{
    "message": "User registered successfully",
    "login": "username"
}
```

#### Аутентификация
```bash
POST /api/user/login
Content-Type: application/json

{
    "login": "username",
    "password": "password"
}
```

**Ответ:**
```json
{
    "message": "Successfully authenticated",
    "login": "username"
}
```

### Защищенные маршруты (требуют Basic Auth)

Все защищенные маршруты требуют заголовок:
```
Authorization: Basic base64(login:password)
```

#### Загрузка файла наблюдений
```bash
POST /api/user/observation
Authorization: Basic base64(login:password)
Content-Type: multipart/form-data

rinex_file: @/path/to/observation.obs
```

**Ответ:**
```json
{
    "message": "File uploaded and queued for processing",
    "file_id": 1,
    "filename": "observation.obs",
    "file_size": 1234567,
    "uploaded_at": "2025-07-14T10:12:31Z",
    "status": "pending"
}
```

#### Получение статуса файла
```bash
GET /api/user/file/{fileID}
Authorization: Basic base64(login:password)
```

**Ответ (в обработке):**
```json
{
    "id": 1,
    "filename": "observation.obs",
    "file_size": 1234567,
    "uploaded_at": "2025-07-14T10:12:31Z",
    "status": "processing",
    "result_id": null
}
```

**Ответ (обработан):**
```json
{
    "id": 1,
    "filename": "observation.obs",
    "file_size": 1234567,
    "uploaded_at": "2025-07-14T10:12:31Z",
    "status": "completed",
    "result_id": 1
}
```

#### Получение результата по ID
```bash
GET /api/user/result/{resultID}
Authorization: Basic base64(login:password)
```

**Ответ:**
```json
{
    "id": 1,
    "file_id": 1,
    "user_login": "username",
    "x": 455582.3821,
    "y": 3639405.2210,
    "z": 5200656.2537,
    "sdx": 3.9804,
    "sdy": 7.7201,
    "sdz": 13.0986,
    "created_at": "2025-07-14T10:12:35Z"
}
```

#### Получение последнего результата
```bash
GET /api/user/last
Authorization: Basic base64(login:password)
```

#### Получение истории всех результатов
```bash
GET /api/user/history
Authorization: Basic base64(login:password)
```

**Ответ:** массив результатов

#### Выход из системы
```bash
POST /api/user/logout
Authorization: Basic base64(login:password)
```

**Ответ:**
```json
{
    "message": "Logged out successfully"
}
```

### Коды ответов

| Код | Описание |
|-----|----------|
| `200 OK` | Успешный запрос |
| `201 Created` | Ресурс успешно создан |
| `202 Accepted` | Файл принят в обработку |
| `400 Bad Request` | Неверный запрос |
| `401 Unauthorized` | Отсутствует или неверная аутентификация |
| `403 Forbidden` | Доступ запрещен (не ваш файл) |
| `404 Not Found` | Ресурс не найден |
| `409 Conflict` | Конфликт (пользователь уже существует) |
| `413 Request Entity Too Large` | Файл слишком большой |
| `500 Internal Server Error` | Внутренняя ошибка сервера |

## 📊 Структура базы данных

### Таблица `users`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `login` | VARCHAR(255) PRIMARY KEY | Логин пользователя |
| `password` | VARCHAR(255) NOT NULL | Хеш пароля (bcrypt) |

### Таблица `observation_files`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | SERIAL PRIMARY KEY | Уникальный ID файла |
| `user_login` | VARCHAR(255) | Логин пользователя (FK) |
| `filename` | VARCHAR(500) | Имя файла |
| `file_size` | BIGINT | Размер в байтах |
| `uploaded_at` | TIMESTAMP | Время загрузки |
| `status` | VARCHAR(50) | Статус обработки |
| `result_id` | INTEGER | ID результата (FK) |

### Таблица `adjustment_results`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | SERIAL PRIMARY KEY | Уникальный ID результата |
| `file_id` | INTEGER | ID файла (FK) |
| `user_login` | VARCHAR(255) | Логин пользователя (FK) |
| `x` | FLOAT8 | Координата X (метры) |
| `y` | FLOAT8 | Координата Y (метры) |
| `z` | FLOAT8 | Координата Z (метры) |
| `sdx` | FLOAT4 | СКО по X |
| `sdy` | FLOAT4 | СКО по Y |
| `sdz` | FLOAT4 | СКО по Z |
| `created_at` | TIMESTAMP | Время создания |

## 🔄 Процесс обработки

1. **Загрузка** - пользователь загружает RINEX файл через API
2. **Валидация** - проверка формата и размера файла
3. **Сохранение** - метаданные сохраняются в БД, файл временно в памяти
4. **Определение даты** - парсинг заголовка RINEX для поиска `TIME OF FIRST OBS`
5. **Скачивание эфемерид** - формирование URL и скачивание с IGS сервера
6. **Обработка** - запуск rnx2rtkp с конфигурацией single.conf
7. **Парсинг результата** - извлечение координат и точностных характеристик
8. **Сохранение** - результат сохраняется в БД
9. **Очистка** - удаление всех временных файлов

## 🧪 Тестирование

### Подготовка тестовой базы данных

```bash
# Создание тестовой БД
make db-test-create

# Очистка тестовой БД
make db-test-drop

# Пересоздание тестовой БД
make db-test-reset
```

### Запуск тестов

```bash
# Все тесты
make test

# Тесты с покрытием
make test-cover

# Unit-тесты
make test-unit

# Интеграционные тесты
make test-integration

# Бенчмарки
make bench

# Линтер
make lint
```

## 📁 Структура проекта

```
GNSS-Service/
├── cmd/
│   ├── gnss-service/
│   │   ├── flags.go  
│   │   └── main.go                     # Точка входа в приложение
│   └── rtklib/
│   │   ├── processor/
│   │   │   └── processor.go            # Обработка RINEX файлов
│       └── app/
│           ├── rnx2rtkp                # Исполняемый файл RTKLIB
│           └── single.conf             # Конфигурация RTKLIB
├── example/
│   ├── GEOP195K.obs
├── internal/
│   ├── handlers/
│   │   ├── auth.go                      # Хендлеры аутентификации
│   │   ├── index.go                      # Главная страница
│   │   └── observations.go               # Хендлеры для файлов и результатов
│   ├── middleware-service/
│   │   └── middlewareservice.go          # Middleware (логирование, аутентификация)
│   └── storage/
│       └── db.go                            # Работа с PostgreSQL
├── tmp/                                      # Временные файлы
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 📝 Примеры использования

### Полный цикл работы

```bash
#!/bin/bash

# Переменные
USER="john"
PASS="secret"
SERVER="http://localhost:8080"
FILE="/path/to/observation.obs"

# 1. Регистрация
echo "Registering user..."
curl -X POST $SERVER/api/user/register \
  -H "Content-Type: application/json" \
  -d "{\"login\": \"$USER\", \"password\": \"$PASS\"}"

# 2. Логин (проверка)
echo -e "\n\nLogging in..."
curl -X POST $SERVER/api/user/login \
  -H "Content-Type: application/json" \
  -d "{\"login\": \"$USER\", \"password\": \"$PASS\"}"

# 3. Загрузка файла
echo -e "\n\nUploading file..."
AUTH=$(echo -n "$USER:$PASS" | base64)
RESPONSE=$(curl -X POST $SERVER/api/user/observation \
  -H "Authorization: Basic $AUTH" \
  -F "rinex_file=@$FILE")

echo "Response: $RESPONSE"

# Извлекаем file_id из ответа
FILE_ID=$(echo $RESPONSE | grep -o '"file_id":[0-9]*' | cut -d':' -f2)

if [ -n "$FILE_ID" ]; then
    echo -e "\n\nFile ID: $FILE_ID"
    
    # 4. Проверка статуса
    echo -e "\n\nChecking status..."
    curl -X GET $SERVER/api/user/file/$FILE_ID \
      -H "Authorization: Basic $AUTH"
    
    # Ждем обработки
    echo -e "\n\nWaiting 10 seconds for processing..."
    sleep 10
    
    # 5. Получение последнего результата
    echo -e "\n\nGetting last result..."
    curl -X GET $SERVER/api/user/last \
      -H "Authorization: Basic $AUTH"
    
    # 6. История всех результатов
    echo -e "\n\nGetting history..."
    curl -X GET $SERVER/api/user/history \
      -H "Authorization: Basic $AUTH"
fi
```

### Работа с curl

```bash
# Сохраняем аутентификацию в переменную
AUTH="Authorization: Basic $(echo -n 'john:secret' | base64)"

# Загрузка файла
curl -X POST http://localhost:8080/api/user/observation \
  -H "$AUTH" \
  -F "rinex_file=@test.obs"

# Проверка статуса
curl -X GET http://localhost:8080/api/user/file/1 -H "$AUTH"

# Получение результата
curl -X GET http://localhost:8080/api/user/result/1 -H "$AUTH"

# Последний результат
curl -X GET http://localhost:8080/api/user/last -H "$AUTH"

# История
curl -X GET http://localhost:8080/api/user/history -H "$AUTH"
```

## ⚡ Производительность

- **Пул воркеров** - до 3 одновременных обработок (настраивается)
- **Ограничение размера файла** - 1 ГБ
- **Таймаут скачивания** - 5 минут
- **Очистка временных файлов** - автоматически после обработки

---

**Happy GNSS Processing!** 🌍🛰️
