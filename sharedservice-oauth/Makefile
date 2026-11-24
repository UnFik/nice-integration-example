SHELL := /bin/bash

.PHONY: run up

run:
	@set -m; \
	$(MAKE) -C backend run & backend=$$!; \
	npm --prefix web run dev & web=$$!; \
	trap 'kill -- -$$backend -$$web 2>/dev/null; wait 2>/dev/null' EXIT; \
	trap 'exit 130' INT; trap 'exit 143' TERM; \
	wait -n $$backend $$web

# Docker: the backend on 8081, the UI (nginx) on 5181 and PostgreSQL (needs the NICE stack, `make up` in nice/).
up:
	docker compose -f ../compose.yml up -d --build
