PROTOC ?= protoc
GOBIN ?= $(shell go env GOPATH)/bin
PROTO_DIR := api/proto
OUT_DIR := api/gen
PROTOS := $(PROTO_DIR)/filesvc/v1/filesvc.proto

.PHONY: proto build run-server client-upload client-download client-list deps

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
	@echo "test content" > test.txt
	go run ./cmd/client -addr localhost:50051 -cmd upload -file test.txt

client-download:
	@mkdir -p downloads
	go run ./cmd/client -addr localhost:50051 -cmd download -file test.txt -out ./downloads/test.txt

client-list:
	go run ./cmd/client -addr localhost:50051 -cmd list

check-download:
	@echo "Checking downloaded file:"
	@cat ./downloads/test.txt

