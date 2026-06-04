.PHONY: up down run build tidy test

up:
	docker compose up -d db

down:
	docker compose down

run:
	go run ./cmd/server

build:
	go build ./...

tidy:
	go mod tidy

test:
	go test ./...
