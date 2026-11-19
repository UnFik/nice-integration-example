#!/usr/bin/env bash
# Smoke test of the service-account-client example against a NICE and a nadi set up with the README checklist: NICE
# answers, nadi answers, and the client lists the devices with a token NICE issued for its Service Account. It reads
# .env only and never prints the key or a token: the client reads the key from .env itself, so it reaches no argv.
# It creates nothing, so it is safe to run again.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

fail() {
  printf 'GAGAL: %s\n' "$1" >&2
  shift
  for line in "$@"; do printf '       %s\n' "$line" >&2; done
  exit 1
}
ok() { printf 'ok    %s\n' "$1"; }

# GO names the go command, as in the Makefile.
go_cmd=${GO:-go}
for cmd in curl "$go_cmd"; do
  command -v "$cmd" >/dev/null || fail "perintah '$cmd' tidak ditemukan; pasang dulu."
done

[[ -f .env ]] || fail ".env tidak ditemukan di $(pwd)." \
  "Salin .env.example ke .env lalu isi Service Account dan API key-nya (checklist README langkah 2 dan 4)."
set -a
# shellcheck source=/dev/null
source ./.env
set +a

[[ -n ${NICE_ISSUER:-} ]] || fail "NICE_ISSUER kosong di .env (checklist langkah 4)."
[[ -n ${NICE_SA_ID:-} ]] || fail "NICE_SA_ID kosong di .env." \
  "Buat Service Account tenant dan isi id-nya (checklist langkah 2)."
[[ -n ${NICE_SA_API_KEY:-} ]] || fail "NICE_SA_API_KEY kosong di .env." \
  "Buat API key Service Account itu dan isi di sini (checklist langkah 4)."
# The client reads the key from .env on its own; nothing below needs it.
unset NICE_SA_API_KEY

nice=${NICE_ISSUER%/}
nadi=${NADI_URL:-http://127.0.0.1:8081}
nadi=${nadi%/}

status=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$nice/.well-known/openid-configuration" || true)
[[ $status == 200 ]] || fail "NICE tidak terjangkau di $nice (HTTP $status)." \
  "Jalankan NICE (cd ../../nice && make dev) dan pastikan NICE_ISSUER di .env sama dengan NICE_ISSUER NICE."
ok "NICE menjawab di $nice"

status=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$nadi/healthz" || true)
[[ $status == 2* ]] || fail "nadi tidak terjangkau di $nadi (HTTP $status)." \
  "Jalankan sharedservice-oauth/backend dengan 'make run' dan periksa NADI_URL (checklist langkah 1)."
ok "nadi menjawab /healthz di $nadi"

out=$(mktemp)
trap 'rm -f "$out"' EXIT
# The client prints its own explanation and the checklist step on stderr when it fails.
"$go_cmd" run ./cmd/service-account-client list >"$out" ||
  fail "perintah list gagal; penjelasan dan langkah checklist ada pada baris FAILED di atas."
ok "list dengan token Service Account: $(($(wc -l <"$out") - 1)) perangkat"
printf 'smoke service-account-client lulus\n'
