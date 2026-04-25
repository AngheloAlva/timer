.PHONY: dev test test-race lint build tidy clean

# Run the CLI directly (will eventually be `go run ./cmd/timer tui`)
dev:
	go run ./cmd/timer

# Unit tests with coverage
test:
	go test ./... -cover

# Tests with race detector — slower, run before pushing
test-race:
	go test ./... -race -cover

lint:
	golangci-lint run

# Compile a static binary into bin/timer
build:
	CGO_ENABLED=0 go build -o bin/timer ./cmd/timer

tidy:
	go mod tidy

clean:
	rm -rf bin/ dist/ coverage.out coverage.html
