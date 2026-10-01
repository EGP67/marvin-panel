# Quality gate is mandatory on every build. make verify must exit 0 before a commit.
GO      ?= go
PKG     := ./...
BIN     := bin/marvind
PREFIX  := /opt/heartofgold

# deploy must be phony: a deploy/ directory exists.
.PHONY: all build test lint vuln secrets verify tools clean shot \
	deploy restart restart-kiosk install-service-prereqs install-service

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
	gitleaks git --pre-commit --staged --redact --exit-code 1 .

# The mandated gate, in the order that fails fastest.
verify: lint test vuln secrets
	@echo "verify: PASS"

tools:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	$(GO) install github.com/zricethezav/gitleaks/v8@latest

# Renders one fixture snapshot for eyeballing; extend to a browser screenshot at T10.
shot: build
	$(BIN) --oneshot --fixture fixtures/busy.json

clean:
	rm -rf bin *.out coverage.txt

# D-052: no sudo. Atomic replace (install to .new, then rename) so a running marvind
# keeps its old inode until restart.
deploy: build
	@test -w $(PREFIX)/bin -a -w $(PREFIX)/fixtures || { echo "$(PREFIX) not writable: run make install-service-prereqs first" >&2; exit 1; }
	install -m 0755 $(BIN) $(PREFIX)/bin/marvind.new
	mv -f $(PREFIX)/bin/marvind.new $(PREFIX)/bin/marvind
	install -m 0755 deploy/heartofgold-kiosk-session $(PREFIX)/bin/heartofgold-kiosk-session.new
	mv -f $(PREFIX)/bin/heartofgold-kiosk-session.new $(PREFIX)/bin/heartofgold-kiosk-session
	install -m 0644 fixtures/*.json $(PREFIX)/fixtures/

# Allowed without sudo by deploy/50-heartofgold.rules (polkit, D-052).
restart:
	systemctl restart marvind.service

restart-kiosk:
	systemctl restart heartofgold-kiosk.service

# Owner-run (sudo); Claude Code only under an explicit per-prompt owner exception.
# Idempotent: account (D-023), /opt/heartofgold owned by marvin, polkit rule.
install-service-prereqs:
	getent passwd heartofgold >/dev/null || sudo useradd --system --no-create-home \
		--shell /usr/sbin/nologin --user-group --groups video,render heartofgold
	sudo install -d -o marvin -g marvin -m 0755 $(PREFIX) $(PREFIX)/bin $(PREFIX)/fixtures
	sudo install -m 0644 deploy/50-heartofgold.rules /etc/polkit-1/rules.d/50-heartofgold.rules

# Owner-run (sudo); Claude Code only under an explicit per-prompt owner exception.
# Archives any installed copy first, installs units and Xorg config, enables only.
install-service:
	@TS=$$(date +%Y%m%d-%H%M%S); \
	for f in /etc/systemd/system/marvind.service /etc/systemd/system/heartofgold-kiosk.service \
		/etc/X11/xorg.conf.d/10-heartofgold.conf; do \
		if [ -e "$$f" ]; then sudo cp -p "$$f" "/srv/hogdata/marvin/archive/$$(basename "$$f").$$TS" && echo "archived $$f"; fi; \
	done
	sudo install -m 0644 deploy/marvind.service deploy/heartofgold-kiosk.service /etc/systemd/system/
	sudo install -d -m 0755 /etc/X11/xorg.conf.d
	sudo install -m 0644 deploy/10-heartofgold.conf /etc/X11/xorg.conf.d/10-heartofgold.conf
	U=$$(id -u heartofgold); D=/etc/systemd/system/user-$$U.slice.d; \
	if [ -e "$$D/50-heartofgold.conf" ]; then sudo cp -p "$$D/50-heartofgold.conf" "/srv/hogdata/marvin/archive/user-slice-50-heartofgold.conf.$$(date +%Y%m%d-%H%M%S)"; fi; \
	sudo install -d -m 0755 "$$D" && sudo install -m 0644 deploy/heartofgold-user-slice.conf "$$D/50-heartofgold.conf"
	@grep -q '^allowed_users=console' /etc/X11/Xwrapper.config || { echo "Xwrapper.config: allowed_users=console missing (D-025); not edited"; exit 1; }
	sudo systemctl daemon-reload
	sudo systemctl disable getty@tty1.service
	sudo systemctl enable marvind.service heartofgold-kiosk.service
	@echo "install-service: units enabled; nothing started"
