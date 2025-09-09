.PHONY: build run test docker

build:
	go build ./...

run:
	go run ./cmd/server

test:
	go test ./... -v

