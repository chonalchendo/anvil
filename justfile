# anvil — task runner

# Build the CLI binary.
build:
    go build -o bin/anvil ./cmd/anvil

# Run all tests.
test:
    go test ./...

# Run lints (golangci-lint v2 — install via tool.go.mod once wired).
lint:
    golangci-lint run

# Run go vet.
vet:
    go vet ./...

# Run the binary.
run:
    go run ./cmd/anvil
