# HEART OF GOLD — Marvin telemetry panel
Portrait 1080x1920 telemetry panel for hog.local, spoken by Marvin.

Start here: HANDOFF.md, then docs/DISCOVERY.md, then TASKS.md.
Read docs/PANEL_BUYING.md before buying the display.

    make tools    # install golangci-lint, govulncheck, gitleaks
    make verify   # the mandatory gate: lint + test -race + vulncheck + gitleaks
    make build    # bin/marvind
    go run ./cmd/marvind --oneshot

Panel not yet purchased. Develop against fixtures; see TASKS.md T2.
