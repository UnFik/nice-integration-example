GO ?= go
SWAG_FLAGS = -g cmd/nadi/main.go -d ./ --parseInternal --outputTypes json --quiet

.PHONY: run test swag swag-check

run:
	$(GO) run ./cmd/nadi

test:
	$(GO) test ./... -count=1

swag:
	$(GO) tool swag init $(SWAG_FLAGS) -o docs

swag-check:
	@tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && \
	$(GO) tool swag init $(SWAG_FLAGS) -o "$$tmp" && \
	if diff -u docs/swagger.json "$$tmp/swagger.json"; then \
		echo "docs/swagger.json is up to date"; \
	else \
		echo "docs/swagger.json is out of date: run 'make swag' and commit the result" >&2; exit 1; \
	fi
