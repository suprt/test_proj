# test_proj

gRPC сервис хранения изображений с потоковой передачей и ограничением конкуренции по IP клиента.

**Возможности:**
- Upload/Download изображений (streaming)
- Список файлов с метаданными
- Лимит конкуренции: 10 upload/download + 100 list запросов на IP

## Быстрый старт

### Docker (рекомендуется)
```bash
# Сборка и запуск
make docker-build
make docker-run

# Интеграционные тесты
make docker-test-all

# Остановка
make docker-stop
```

### Локально
```bash
# Установка зависимостей
make deps

# Генерация gRPC кода
make proto

# Запуск сервера
make run-server

# Тесты клиента (в другом терминале)
make client-upload
make client-list
make client-download
```

## Команды

### Docker тесты
- `make docker-test-upload` — загрузить тестовое изображение
- `make docker-test-list` — показать список файлов  
- `make docker-test-download` — скачать изображение
- `make docker-check-download` — проверить целостность
- `make docker-test-clean` — очистить артефакты

### Отладка
- `make docker-logs` — логи сервисов
- `make docker-shell` — shell в контейнере сервера
- `docker compose exec client sh` — shell в контейнере клиента

## Требования
- **Docker:** Engine 20.10+, Compose v2
- **Локально:** Go 1.24.6+, protoc 3.21+

