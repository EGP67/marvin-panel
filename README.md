# HEART OF GOLD — Marvin telemetry panel
Portrait 1080x1920 telemetry panel for hog.local, spoken by Marvin.

Start here: CLAUDE.md (agent rules), then HANDOFF.md, docs/DECISIONS.md, TASKS.md.
Read docs/PANEL_BUYING.md before buying the display.

    make tools    # install golangci-lint, govulncheck, gitleaks
    make verify   # the mandatory gate: lint + test -race + vulncheck + gitleaks
    make build    # bin/marvind
    go run ./cmd/marvind --oneshot --fixture fixtures/busy.json

Interim display: SunFounder 7" HDMI panel (D-044). Until the collectors exist (T4-T8),
develop against fixtures; see TASKS.md T2 and T2b.
