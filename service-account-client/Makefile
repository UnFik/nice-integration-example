GO ?= go
# The command and its flags, e.g. make run ARGS="create -name 'Sensor Suhu' -serial SN-1 -status active"
ARGS ?= list

.PHONY: run test smoke

run:
	$(GO) run ./cmd/service-account-client $(ARGS)

test:
	$(GO) test -race ./... -count=1

smoke:
	GO=$(GO) scripts/smoke.sh
