# anvil — task runner

# Build the CLI binary.
build:
    go build -o bin/anvil ./cmd/anvil

# Run all tests.
test:
    go test ./...

# Run lints (golangci-lint v2 via tool.go.mod).
lint:
    go tool -modfile=tool.go.mod golangci-lint run ./...

# Run go vet.
vet:
    go vet ./...

# Run the binary.
run:
    go run ./cmd/anvil

# Install anvil into $(go env GOPATH)/bin and assert it's what PATH resolves to.
# Closes the stale-binary smoke-test gap: if an older anvil is earlier on PATH,
# `go install` succeeds but every subsequent `anvil` call runs the stale copy.
# `-a` forces a full rebuild so edits to //go:embed sources (e.g. SKILL.md
# bodies under skills/) reach the installed binary's embed.FS — without it
# Go's build cache can reuse an object whose embed snapshot predates the edit.
install:
    #!/usr/bin/env bash
    set -euo pipefail
    go install -a ./cmd/anvil
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

# Run all local checks in CI order: fmt, lint, vet, build, test, init+validate smoke. Mirrors .github/workflows/ci.yml.
check:
    #!/usr/bin/env bash
    set -euo pipefail
    fmt_out="$(gofmt -l .)"
    if [ -n "$fmt_out" ]; then
        echo "gofmt findings:" >&2
        echo "$fmt_out" >&2
        exit 1
    fi
    go tool -modfile=tool.go.mod golangci-lint run ./...
    go vet ./...
    go build ./...
    go test ./...
    vault_dir="$(mktemp -d)"
    trap 'rm -rf "$vault_dir"' EXIT
    go run ./cmd/anvil init "$vault_dir"
    go run ./cmd/anvil validate "$vault_dir"

# Install pre-commit hooks via prek. Requires `brew install j178/tap/prek` first.
install-hooks:
    prek install
