PROTOC ?= protoc
GOBIN ?= $(shell go env GOPATH)/bin
PROTO_DIR := api/proto
OUT_DIR := api/gen
PROTOS := $(PROTO_DIR)/filesvc/v1/filesvc.proto

.PHONY: proto build run-server client-upload client-download client-list deps docker-build docker-run docker-stop docker-logs docker-shell docker-test-upload docker-test-list docker-test-download docker-check-download

deps:
	@echo "Installing protoc plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.1
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.4.0

proto: $(PROTOS)
	@mkdir -p $(OUT_DIR)
	PATH="$(GOBIN):$$PATH" $(PROTOC) -I $(PROTO_DIR) \
	  --go_out=$(OUT_DIR) --go_opt=paths=source_relative \
	  --go-grpc_out=$(OUT_DIR) --go-grpc_opt=paths=source_relative \
	  $(PROTOS)

build:
	go build ./...

run-server:
	go run ./cmd/server -addr :50051 -data ./data

client-upload:
	@echo "Creating test file..."
	@dd if=/dev/urandom of=test-image.jpg bs=1024 count=5 2>/dev/null
	go run ./cmd/client -addr localhost:50051 -cmd upload -file test-image.jpg

client-download:
	@mkdir -p downloads
	go run ./cmd/client -addr localhost:50051 -cmd download -file test-image.jpg -out ./downloads/test-image.jpg

client-list:
	go run ./cmd/client -addr localhost:50051 -cmd list

check-download:
	@echo "Checking downloaded file:"
	@file ./downloads/test-image.jpg
	@ls -lh ./downloads/test-image.jpg

# Docker commands
docker-build:
	docker compose build

docker-run:
	docker compose up -d

docker-stop:
	docker compose down

docker-logs:
	docker compose logs -f

docker-shell:
	docker compose exec filesvc sh

# Test with Docker
docker-test-upload:
	@echo "Creating test file in client container and uploading to filesvc..."
	docker compose exec client sh -lc 'dd if=/dev/urandom of=/app/test-image.jpg bs=1024 count=10 && ./client -addr filesvc:50051 -cmd upload -file /app/test-image.jpg'

docker-test-list:
	docker compose exec client ./client -addr filesvc:50051 -cmd list

docker-test-download:
	docker compose exec client ./client -addr filesvc:50051 -cmd download -file test-image.jpg -out /app/downloaded-test-image.jpg

docker-check-download:
	@echo "Checking downloaded image inside client container:"
	docker compose exec client sh -lc 'echo "Downloaded image info:" && file /app/downloaded-test-image.jpg && ls -lh /app/downloaded-test-image.jpg && echo "Original image info:" && file /app/test-image.jpg && ls -lh /app/test-image.jpg'

# Aggregate and cleanup targets
docker-test-all: docker-test-upload docker-test-list docker-test-download docker-check-download
	@echo "All docker client-server tests completed"

docker-test-clean:
	@echo "Cleaning test artifacts in client container"
	docker compose exec client sh -lc 'rm -f /app/test-image.jpg /app/downloaded-test-image.jpg || true'
	@echo "Cleaning test artifacts on host"
	@rm -f ./data/test-image.jpg ./data/test-image.jpg.meta.json || true


