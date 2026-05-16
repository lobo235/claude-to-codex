BIN := bin/claude-to-codex
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build test lint cover vuln run install uninstall clean inspect inspect-tools

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

run: build
	$(BIN) serve

install: build
	install -d $(HOME)/.local/bin
	install -m 0755 $(BIN) $(HOME)/.local/bin/claude-to-codex
	install -m 0755 scripts/codex-with-claude $(HOME)/.local/bin/codex-with-claude

uninstall:
	rm -f $(HOME)/.local/bin/claude-to-codex $(HOME)/.local/bin/codex-with-claude

clean:
	rm -rf bin

inspect: build
	$(BIN) inspect

inspect-tools: build
	$(BIN) inspect --tools
