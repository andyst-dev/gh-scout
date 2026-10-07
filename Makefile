.PHONY: build test vet lint fmt fmt-check clean

BIN     := bin/gh-scout
VERSION ?= dev

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BIN) ./cmd/gh-scout

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "run 'make fmt'"; exit 1; }

clean:
	rm -rf bin