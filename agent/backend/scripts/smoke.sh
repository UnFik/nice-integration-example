#!/usr/bin/env bash
# Smoke test of the agen example against a NICE set up with the README checklist: the agent answers, /mcp without a
# token points to the resource metadata, the metadata names NICE, and, with AGENT_USER_TOKEN in the environment
# (checklist step 10), the three tools are listed, list_services lists nadi, list_operations lists GET /devices of
# nadi, and call_operation runs it through a token exchange. AGENT_URL points it at an agent elsewhere.
# It reads .env, AGENT_URL, and AGENT_USER_TOKEN only and never prints a token or the secret; neither reaches the
# argv of curl.
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
[[ -f .env ]] || fail ".env tidak ditemukan di $(pwd)." "Salin .env.example ke .env lalu isi client agen-backend (checklist langkah 7)."
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
agent=${AGENT_URL:-http://$listen}
agent=${agent%/}

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
[[ $status == 200 ]] || fail "agen tidak terjangkau di $agent (HTTP $status)." "Jalankan agen dengan 'make run' (checklist langkah 9)."
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
  printf 'lewat  uji tool: AGENT_USER_TOKEN kosong (checklist langkah 10)\n'
  exit 0
fi

status=$(mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')
if [[ $status != 200 ]]; then
  reason=$(jq -r '.error_description // empty' "$body" 2>/dev/null || true)
  case $status:$reason in
    401:wrong_audience) fail "token user bukan untuk agen (401 wrong_audience)." "Minta token lewat client agen-user dengan scope nice:aud:agen (checklist langkah 6 dan 10)." ;;
    401:*) fail "token user ditolak: 401 $reason." "Ambil token baru (checklist langkah 10)." ;;
    403:*) fail "token user tanpa agen.mcp.use: 403 $reason." "Beri user role agen.user di project token (checklist langkah 8)." ;;
    *) fail "tools/list dijawab HTTP $status." ;;
  esac
fi
tools=$(jq -r '[.result.tools[].name] | sort | join(",")' "$body")
[[ $tools == call_operation,list_operations,list_services ]] ||
  fail "tools/list memuat $tools, seharusnya list_services, list_operations, dan call_operation."
ok "tools/list: tiga tool"

# tool <id> <name> <arguments JSON> calls one tool and stops with a hint for the step a refusal points to.
tool() {
  status=$(mcp "$(jq -nc --argjson id "$1" --arg name "$2" --argjson args "$3" \
    '{jsonrpc: "2.0", id: $id, method: "tools/call", params: {name: $name, arguments: $args}}')")
  [[ $status == 200 ]] || fail "$2 dijawab HTTP $status."
  [[ $(jq -r '.result.isError // false' "$body") == true ]] || return 0
  text=$(jq -r '.result.content[0].text' "$body")
  case $text in
    discovery_unavailable*) fail "$text" "NICE terlalu lama: naikkan ke revisi yang disebut README (Versi NICE minimum)." ;;
    service_not_allowed*) fail "$text" "User belum boleh memanggil nadi: langkah 6 sampai 8 README sharedservice-oauth/backend." ;;
    insecure_endpoint*) fail "$text" "Daftarkan endpoint nadi di katalog sebagai https, atau loopback untuk uji lokal." ;;
    invalid_client*) fail "$text" "Periksa NICE_CLIENT_ID dan NICE_CLIENT_SECRET (checklist langkah 7)." ;;
    invalid_scope*) fail "$text" "Periksa bahwa nadi aktif di project token (langkah 7 README sharedservice-oauth/backend)." ;;
    invalid_grant*) fail "$text" "Token user hampir habis, basi, atau sesinya berakhir: ambil token baru (checklist langkah 10)." ;;
    *) fail "$text" ;;
  esac
}

tool 2 list_services '{}'
jq -e '.result.structuredContent.services | map(.slug) | index("nadi")' "$body" >/dev/null ||
  fail "list_services tidak memuat nadi." "User belum memegang izin nadi di project mana pun: langkah 8 README sharedservice-oauth/backend."
ok "list_services: nadi di $(jq '[.result.structuredContent.organizations[].projects[] | select(.services | index("nadi"))] | length' "$body") project"

tool 3 list_operations '{"service":"nadi"}'
key=$(jq -r '.result.structuredContent.operations[] | select(.method == "GET" and .path == "/devices") | .key' "$body" | head -1)
[[ -n $key ]] || fail "list_operations nadi tidak memuat GET /devices." "Impor ulang katalog nadi dan pastikan user memegang nadi.devices.read."
ok "list_operations nadi: $(jq '.result.structuredContent.operations | length' "$body") operasi, GET /devices = $key"

tool 4 call_operation "$(jq -nc --arg key "$key" '{service: "nadi", operation: $key}')"
ok "call_operation $key lewat token exchange: HTTP $(jq -r '.result.structuredContent.status' "$body"), $(jq -r '.result.structuredContent.body.devices | length' "$body") perangkat"
