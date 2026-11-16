#!/usr/bin/env bash
# Smoke test: the running service and a NICE set up with the README checklist.
# Reads .env only. Never prints the service credential, the API key, or the token.
#
#   scripts/smoke.sh
set -euo pipefail

cd "$(dirname "$0")/.."

fail() {
  printf 'GAGAL: %s\n' "$1" >&2
  if [ -n "${2:-}" ]; then printf '       Periksa README, langkah %s.\n' "$2" >&2; fi
  exit 1
}
ok() { printf 'ok    %s\n' "$1"; }

for tool in curl jq; do
  command -v "$tool" >/dev/null || fail "$tool tidak terpasang"
done

[ -f .env ] || fail ".env tidak ada: salin .env.example ke .env lalu isi" 2
set -a
# shellcheck disable=SC1091
. ./.env
set +a

nice=${NICE_URL:-http://localhost:8080}
nice=${nice%/}
service="http://${LISTEN_ADDR:-127.0.0.1:8082}"
[ -n "${SMOKE_SA_ID:-}" ] || fail "SMOKE_SA_ID kosong di .env" 9
[ -n "${SMOKE_SA_API_KEY:-}" ] || fail "SMOKE_SA_API_KEY kosong di .env" 11

body=$(mktemp)
trap 'rm -f "$body"' EXIT

# call sets $status and leaves the response body in $body; 000 means no answer.
# Secrets reach curl on stdin (-H @- or --data-urlencode name@-), never in its arguments. Feed stdin with
# < <(printf ...), not a pipe: a pipe would run call in a subshell and lose $status.
call() {
  : >"$body"
  status=$(curl -s -o "$body" -w '%{http_code}' --max-time 15 "$@") || status=000
}

# field prints a top-level string field of the JSON body, or nothing.
field() { jq -r --arg f "$1" 'if type == "object" then (.[$f] // "") | tostring else "" end' "$body" 2>/dev/null || true; }

describe() {
  local code
  code=$(field error_code)
  [ -n "$code" ] || code=$(field error)
  printf 'HTTP %s%s' "$status" "${code:+ $code}"
}

# 1. Service reachable and guarded.
call "$service/healthz" </dev/null
[ "$status" = 200 ] || fail "service tidak menjawab di $service/healthz ($(describe)); jalankan npm run dev" 2
call "$service/shipments" </dev/null
[ "$status" = 401 ] || fail "panggilan langsung tanpa X-Api-Key harus 401, didapat $(describe)" 2
ok "langsung ke service tanpa key: 401"

# 2. NICE reachable.
call "$nice/healthz" </dev/null
case $status in
  2??) ;;
  *) fail "NICE tidak terjangkau di $nice ($(describe)); jalankan make dev di nice/ dan periksa NICE_URL" 1 ;;
esac

# 3. Proxy route exists and asks for a token.
call "$nice/pengiriman/shipments" </dev/null
case $status in
  401) ok "proxy tanpa token: 401" ;;
  404) fail "proxy tidak mengenal GET /pengiriman/shipments ($(describe)): service belum terdaftar sebagai api_key atau katalog belum diimpor" "4-5" ;;
  503) fail "proxy menolak ($(describe)): service belum aktif" 6 ;;
  *) fail "proxy tanpa token harus 401, didapat $(describe)" "4-6" ;;
esac

# 4. Service Account token for the pengiriman audience.
call "$nice/oauth/token" \
  --data-urlencode grant_type=client_credentials \
  --data-urlencode "client_id=$SMOKE_SA_ID" \
  --data-urlencode client_secret@- \
  --data-urlencode scope=nice:aud:pengiriman < <(printf '%s' "$SMOKE_SA_API_KEY")
if [ "$status" != 200 ]; then
  reason=$(field error)
  detail=$(field error_description)
  case $reason in
    invalid_client) fail "token ditolak invalid_client: SMOKE_SA_ID atau SMOKE_SA_API_KEY salah, key dicabut atau kedaluwarsa, atau Service Account nonaktif" "9, 11" ;;
    invalid_scope) fail "token ditolak invalid_scope${detail:+ ($detail)}: service belum aktif, organisasi belum di-entitle, project belum mengaktifkan pengiriman, atau Service Account tidak punya binding ke role pengiriman" "6, 7, 8, 11" ;;
    *) fail "permintaan token gagal: $(describe)${detail:+ ($detail)}" "9-11" ;;
  esac
fi
token=$(jq -r '.access_token // empty' "$body")
[ -n "$token" ] || fail "jawaban token tidak memuat access_token"
ok "token Service Account (scope nice:aud:pengiriman)"

# 5. The call through the proxy.
call "$nice/pengiriman/shipments" -H @- < <(printf 'Authorization: Bearer %s\n' "$token")
if [ "$status" = 200 ]; then
  jq -e '.shipments | type == "array"' "$body" >/dev/null || fail "jawaban 200 tanpa daftar shipments"
  ok "GET $nice/pengiriman/shipments lewat proxy: 200"
  printf 'smoke lulus\n'
  exit 0
fi

code=$(field error_code)
case "$status:$code" in
  401:) fail "service menolak kredensial dari NICE ($(describe)): nilai upstream.credential di NICE berbeda dengan SERVICE_CREDENTIAL, atau (setelah langkah 12) kunci Service Account tidak ada di ${KEYS_FILE:-data/keys.json}" "4, 12" ;;
  401:*) fail "proxy menolak token ($(describe)); jalankan ulang skrip, token baru diambil setiap kali" ;;
  403:permission_denied) fail "Service Account tidak memegang pengiriman.shipments.read ($(describe)): binding belum dibuat, atau scopes API key memotongnya" "10, 11" ;;
  403:service_account_required) fail "proxy hanya menerima Service Account ($(describe))" 12 ;;
  403:*) fail "proxy menolak ($(describe))" "7-11" ;;
  404:*) fail "proxy tidak mengenal operasi ini ($(describe)): impor ulang /openapi.json" 5 ;;
  503:service_disabled) fail "service belum aktif ($(describe))" 6 ;;
  503:upstream_key_unavailable) fail "kunci Service Account belum aktif ($(describe)): tunggu sapuan NICE (sekitar 30 detik) dan periksa log service" 12 ;;
  503:*) fail "proxy tidak meneruskan ($(describe)): service dianggap tidak sehat atau penuh" "2, 4" ;;
  502:* | 504:*) fail "NICE tidak bisa menghubungi service ($(describe)): periksa NICE_OUTBOUND_ALLOW_HTTP/NICE_OUTBOUND_ALLOW_CIDRS, endpoint http://127.0.0.1:8082, dan service berjalan" "1, 2, 4" ;;
  000:*) fail "NICE tidak menjawab panggilan proxy" 1 ;;
  *) fail "panggilan lewat proxy gagal: $(describe)" ;;
esac
