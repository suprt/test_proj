# test_proj

gRPC сервис хранения бинарных файлов (изображений):
- загрузка (стрим): сохраняет файл на диск
- список файлов: Имя | Дата создания | Дата обновления
- скачивание (стрим)
- ограничение конкуренции на клиента (по IP):
  - Upload/Download: 10 конкурентных запросов
  - ListFiles: 100 конкурентных запросов

## Требования
- Go 1.22+
- protoc 3.21+ установлен в системе
- Плагины protoc:
  - `protoc-gen-go` v1.34.1
  - `protoc-gen-go-grpc` v1.4.0

Установка плагинов:
```bash
make deps
```

## Генерация gRPC кода
```bash
make proto
```
Код генерируется в `api/gen/filesvc/v1`.

## Сборка
```bash
make build
```

## Запуск сервера
```bash
make run-server
# или вручную:
# go run ./cmd/server -addr :50051 -data ./data
```

## Клиент
- Загрузка:
```bash
make client-upload
# или:
# go run ./cmd/client -addr localhost:50051 -cmd upload -file ./path/to/file.jpg
```

- Список:
```bash
make client-list
# или:
# go run ./cmd/client -addr localhost:50051 -cmd list
```

- Скачивание:
```bash
make client-download
# или:
# go run ./cmd/client -addr localhost:50051 -cmd download -file file.jpg -out /tmp/file.jpg
```

## Структура проекта
- `api/proto/filesvc/v1/filesvc.proto` — описание API
- `api/gen/filesvc/v1` — сгенерированный код
- `internal/storage` — файловое хранилище (+метаданные)
- `internal/limiter` — ограничитель конкуренции per-client
- `internal/server` — реализация gRPC сервиса
- `cmd/server`, `cmd/client` — бинарники
