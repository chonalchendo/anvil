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

# Install anvil into $(go env GOPATH)/bin and assert it's what PATH resolves to.
# Closes the stale-binary smoke-test gap: if an older anvil is earlier on PATH,
# `go install` succeeds but every subsequent `anvil` call runs the stale copy.
install:
    #!/usr/bin/env bash
    set -euo pipefail
    go install ./cmd/anvil
    gobin="$(go env GOPATH)/bin/anvil"
    path_anvil="$(command -v anvil 2>/dev/null || true)"
    if [ -z "$path_anvil" ]; then
        echo "error: anvil not on PATH after install. Add $(dirname "$gobin") to PATH." >&2
        exit 1
    fi
    if ! [ "$path_anvil" -ef "$gobin" ]; then
        echo "error: 'anvil' on PATH ($path_anvil) is not the just-installed binary ($gobin)." >&2
        echo "  Smoke-tests would run a stale binary. Fix PATH order or repoint the symlink:" >&2
        echo "    ln -sf \"$gobin\" \"$path_anvil\"" >&2
        exit 1
    fi
    echo "installed: $gobin"
    echo "(if your shell has a stale 'anvil' path cached, run: hash -r)"

# Validate vault frontmatter against schemas.
validate vault="":
    @if [ -z "{{vault}}" ]; then go run ./cmd/anvil validate; else go run ./cmd/anvil validate {{vault}}; fi
