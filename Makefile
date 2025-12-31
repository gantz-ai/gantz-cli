.PHONY: build run clean install test

BINARY_NAME=gantz
VERSION=0.1.0

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY_NAME) ./cmd/gantz

run: build
	./bin/$(BINARY_NAME) serve

run-local: build
	./bin/$(BINARY_NAME) serve --local

install: build
	cp bin/$(BINARY_NAME) /usr/local/bin/

clean:
	rm -rf bin/

test:
	go test -v ./...

deps:
	go mod download
	go mod tidy

# Build for multiple platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/gantz
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/gantz
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/gantz
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/gantz
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/gantz
