# Gocaster Makefile

# Build the application
all: build test

build:
	@echo "Building..."
	@go build -o bin/gocaster ./cmd/gocaster/

# Run the application
run:
	@go run ./cmd/gocaster/

debug-run:
	@env DEBUG=true go run ./cmd/gocaster/

# Test the application
test:
	@echo "Testing..."
	@go test ./... -v

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Lint the code
lint:
	@echo "Linting..."
	@golangci-lint run

# Format the code
format:
	@echo "Formatting..."
	@go fmt ./...

# Vet the code
vet:
	@echo "Vetting..."
	@go vet ./...


# Check code quality
check: fmt vet lint test

# Clean test artifacts
clean-test:
	@echo "Cleaning test artifacts..."
	@rm -f coverage.out coverage.html

clean: clean-test
	@echo "Cleaning..."
	@rm -f bin/gocaster

.PHONY: all build run test test-coverage lint format vet check clean clean-test

