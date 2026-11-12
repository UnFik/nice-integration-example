#!/usr/bin/env bash
# Smoke test of the agen example against a NICE set up with the README checklist: the agent answers, /mcp without a
# token points to the resource metadata, the metadata names NICE, and, with AGENT_USER_TOKEN in the environment
# (checklist step 11), the four tools are listed and inventaris_list_items succeeds through a token exchange.
# It reads .env and AGENT_USER_TOKEN only and never prints a token or the secret; neither reaches the argv of curl.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

fail() {
  printf 'GAGAL: %s\n' "$1" >&2
  shift
  for line in "$@"; do printf '       %s\n' "$line" >&2; done
  exit 1
}
ok() { printf 'ok    %s\n' "$1"; }

for cmd in curl jq; do
  command -v "$cmd" >/dev/null || fail "perintah '$cmd' tidak ditemukan; pasang dulu."
done

user_token=${AGENT_USER_TOKEN:-}
[[ -f .env ]] || fail ".env tidak ditemukan di $(pwd)." "Salin .env.example ke .env lalu isi client agen-backend (checklist langkah 8)."
set -a
# shellcheck source=/dev/null
source ./.env
set +a
unset NICE_CLIENT_SECRET

[[ -n ${NICE_ISSUER:-} ]] || fail "NICE_ISSUER kosong di .env."
nice=${NICE_ISSUER%/}
listen=${LISTEN_ADDR:-127.0.0.1:8083}
case $listen in
  :*) listen="127.0.0.1$listen" ;;
  0.0.0.0:*) listen="127.0.0.1:${listen#0.0.0.0:}" ;;
esac
agent="http://$listen"

body=$(mktemp)
headers=$(mktemp)
trap 'rm -f "$body" "$headers"' EXIT

# req prints the HTTP status (000 when unreachable) and leaves the body in $body and the headers in $headers.
req() { curl -s -o "$body" -D "$headers" -w '%{http_code}' --max-time 20 "$@" || true; }
# mcp sends one JSON-RPC message with the user token, given to curl as a header file, not as an argument.
mcp() {
  req "$agent/mcp" -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
    -H @<(printf 'Authorization: Bearer %s\n' "$user_token") -d "$1"
}

status=$(req "$nice/.well-known/openid-configuration")
[[ $status == 200 ]] || fail "NICE tidak terjangkau di $nice (HTTP $status)." "Jalankan NICE dan pastikan NICE_ISSUER di .env sama dengan issuer NICE."
ok "NICE menjawab di $nice"

status=$(req "$agent/healthz")
[[ $status == 200 ]] || fail "agen tidak terjangkau di $agent (HTTP $status)." "Jalankan agen dengan 'make run' (checklist langkah 10)."
ok "agen menjawab /healthz di $agent"

status=$(req "$agent/mcp" -X POST -H 'Content-Type: application/json' -d '{}')
challenge=$(tr -d '\r' <"$headers" | sed -n 's/^[Ww][Ww][Ww]-[Aa]uthenticate: //p')
[[ $status == 401 && $challenge == *resource_metadata=* ]] ||
  fail "/mcp tanpa token dijawab HTTP $status, seharusnya 401 dengan resource_metadata di WWW-Authenticate."
ok "/mcp tanpa token: 401 dengan resource_metadata"

status=$(req "$agent/.well-known/oauth-protected-resource")
[[ $status == 200 ]] && jq -e --arg iss "$nice" '.authorization_servers | index($iss)' "$body" >/dev/null ||
  fail "metadata resource tidak menyebut $nice sebagai authorization server (HTTP $status)."
ok "metadata resource menunjuk $nice"

if [[ -z $user_token ]]; then
  printf 'lewat  uji tool: AGENT_USER_TOKEN kosong (checklist langkah 11)\n'
  exit 0
fi

status=$(mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')
if [[ $status != 200 ]]; then
  reason=$(jq -r '.error_description // empty' "$body" 2>/dev/null || true)
  case $status:$reason in
    401:wrong_audience) fail "token user bukan untuk agen (401 wrong_audience)." "Minta token lewat client agen-user dengan scope nice:aud:agen (checklist langkah 7 dan 11)." ;;
    401:*) fail "token user ditolak: 401 $reason." "Ambil token baru (checklist langkah 11)." ;;
    403:*) fail "token user tanpa agen.mcp.use: 403 $reason." "Beri user role agen.user di project token (checklist langkah 9)." ;;
    *) fail "tools/list dijawab HTTP $status." ;;
  esac
fi
tools=$(jq -r '[.result.tools[].name] | sort | join(",")' "$body")
[[ $tools == inventaris_create_item,inventaris_list_items,pengiriman_create_shipment,pengiriman_list_shipments ]] ||
  fail "tools/list memuat $tools, seharusnya empat tool contoh."
ok "tools/list: empat tool"

status=$(mcp '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"inventaris_list_items","arguments":{}}}')
[[ $status == 200 ]] || fail "tools/call dijawab HTTP $status."
if [[ $(jq -r '.result.isError // false' "$body") == true ]]; then
  text=$(jq -r '.result.content[0].text' "$body")
  case $text in
    invalid_client*) fail "$text" "Periksa NICE_CLIENT_ID dan NICE_CLIENT_SECRET (checklist langkah 8)." ;;
    invalid_scope*) fail "$text" "Periksa dependensi agen (langkah 4) dan aktivasi inventaris di project token." ;;
    invalid_grant*) fail "$text" "Token user hampir habis, basi, atau sesinya berakhir: ambil token baru (checklist langkah 11)." ;;
    *) fail "$text" ;;
  esac
fi
ok "inventaris_list_items lewat token exchange: $(jq -r '.result.structuredContent.items | length' "$body") barang"
