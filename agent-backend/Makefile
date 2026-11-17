GO ?= go

.PHONY: run test smoke

run:
	$(GO) run ./cmd/agent-backend

test:
	$(GO) test -race ./... -count=1

smoke:
	scripts/smoke.sh
