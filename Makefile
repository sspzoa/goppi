.PHONY: build install test fmt

build:
	go build -o bin/goppi ./cmd/goppi

install:
	go install ./cmd/goppi

test:
	go test ./...

fmt:
	go fmt ./...
