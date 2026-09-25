# Quality gate is mandatory on every build. make verify must exit 0 before a commit.
GO      ?= go
PKG     := ./...
BIN     := bin/marvind

.PHONY: all build test lint vuln secrets verify tools hooks clean shot

all: verify build

build:
	$(GO) vet $(PKG)
	$(GO) build -trimpath -o $(BIN) ./cmd/marvind

test:
	$(GO) test -race -count=1 $(PKG)

lint:
	golangci-lint run $(PKG)

vuln:
	govulncheck $(PKG)

secrets:
	gitleaks detect --source . --redact --exit-code 1

# The mandated gate, in the order that fails fastest.
verify: lint test vuln secrets
	@echo "verify: PASS"

tools:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GO) install github.com/gitleaks/gitleaks/v8@latest

# Blocks pushes that skipped the gate. Install once per clone.
hooks:
	printf '#!/bin/sh\nmake verify || { echo "push blocked: make verify failed"; exit 1; }\n' > .git/hooks/pre-push
	chmod +x .git/hooks/pre-push

# Renders one fixture snapshot for eyeballing; extend to a browser screenshot at T10.
shot: build
	$(BIN) --oneshot --fixture fixtures/busy.json

clean:
	rm -rf bin *.out coverage.txt
