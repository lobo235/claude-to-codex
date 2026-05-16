BIN := bin/claude-to-codex
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)
PREFIX ?= $(HOME)/.local
BINDIR := $(PREFIX)/bin

.PHONY: build test lint cover vuln secrets run install uninstall uninstall-dry-run clean inspect inspect-tools

build:
	go build -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	go test ./...

lint:
	go vet ./...

cover:
	go test -cover ./...

vuln:
	go tool govulncheck ./...

secrets:
	@if ! command -v pre-commit >/dev/null 2>&1; then \
		printf 'pre-commit is not installed. Install it, then run: pre-commit install\n' >&2; \
		exit 127; \
	fi
	pre-commit run gitleaks --all-files

run: build
	$(BIN) serve

install: build
	install -d $(BINDIR)
	install -m 0755 $(BIN) $(BINDIR)/claude-to-codex
	install -m 0755 scripts/codex-with-claude $(BINDIR)/codex-with-claude
	install -m 0755 scripts/codex-with-claude $(BINDIR)/cwc

uninstall:
	PREFIX="$(PREFIX)" ./uninstall.sh --yes

uninstall-dry-run:
	PREFIX="$(PREFIX)" ./uninstall.sh --dry-run

clean:
	rm -rf bin

inspect: build
	$(BIN) inspect

inspect-tools: build
	$(BIN) inspect --tools
