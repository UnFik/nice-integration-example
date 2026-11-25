#!/usr/bin/env bash
# Smoke test of the nadi example against a NICE set up with the README
# checklist: 401 without a token, 200 on GET /devices with a Service Account token
# (client_credentials, scope nice:aud:nadi), 401 on GET /auth/session
# without a cookie, and GET /auth/login redirecting to NICE. It reads only .env
# and never prints a key or a token; secrets never reach the argv of curl or jq.
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

[[ -f .env ]] || fail ".env tidak ditemukan di $(pwd)." \
  "Salin .env.example ke .env lalu isi kuncinya (checklist README langkah 2 dan 9)."
set -a
# shellcheck source=/dev/null
source ./.env
set +a

[[ -n ${NICE_ISSUER:-} ]] || fail "NICE_ISSUER kosong di .env (checklist langkah 2)."
[[ -n ${NICE_CLIENT_ID:-} && -n ${NICE_CLIENT_SECRET:-} ]] ||
  fail "NICE_CLIENT_ID atau NICE_CLIENT_SECRET kosong di .env." \
    "Buat OAuth client confidential untuk UI dan isi keduanya (checklist langkah 2)."
[[ -n ${SMOKE_SA_ID:-} && -n ${SMOKE_API_KEY:-} ]] ||
  fail "SMOKE_SA_ID atau SMOKE_API_KEY kosong di .env." \
    "Buat Service Account tenant beserta API key-nya (checklist langkah 9)."

nice=${NICE_ISSUER%/}
listen=${LISTEN_ADDR:-127.0.0.1:8081}
case $listen in
  :*) listen="127.0.0.1$listen" ;;
  0.0.0.0:*) listen="127.0.0.1:${listen#0.0.0.0:}" ;;
esac
service="http://$listen"

body=$(mktemp)
trap 'rm -f "$body"' EXIT

# get URL prints the HTTP status (000 when unreachable) and leaves the body in $body.
get() { curl -s -o "$body" -w '%{http_code}' --max-time 10 "$@" || true; }
field() { jq -r --arg f "$1" '.[$f] // empty' "$body" 2>/dev/null || true; }

status=$(get "$nice/.well-known/openid-configuration")
[[ $status == 200 ]] || fail "NICE tidak terjangkau di $nice (HTTP $status)." \
  "Jalankan NICE (cd ../../../nice && make dev) dan pastikan NICE_ISSUER di .env sama dengan NICE_ISSUER NICE."
ok "NICE menjawab di $nice"

status=$(get "$service/healthz")
[[ $status == 2* ]] || fail "service tidak terjangkau di $service (HTTP $status)." \
  "Jalankan service dengan 'make run' (checklist langkah 2)."
ok "service menjawab /healthz di $service"

status=$(get "$service/auth/session")
[[ $status == 401 && $(field error) == unauthenticated ]] ||
  fail "GET /auth/session tanpa cookie dijawab HTTP $status $(field error), seharusnya 401 unauthenticated."
ok "GET /auth/session tanpa cookie: 401 unauthenticated"

# The redirect is not followed: only where it points is checked.
login=$(curl -s -o /dev/null -w '%{http_code} %{redirect_url}' --max-time 10 "$service/auth/login" || true)
case $login in
  "302 $nice/oauth/authorize?"*) ;;
  *) fail "GET /auth/login dijawab ${login%% *}, seharusnya 302 ke $nice/oauth/authorize." \
       "Periksa NICE_ISSUER dan NICE_CLIENT_ID di .env (checklist langkah 2)." ;;
esac
[[ $login == *"client_id=${NICE_CLIENT_ID}"* || $login == *"client_id=$(jq -rn --arg v "$NICE_CLIENT_ID" '$v|@uri')"* ]] ||
  fail "GET /auth/login tidak membawa client_id dari NICE_CLIENT_ID (checklist langkah 2)."
ok "GET /auth/login mengarahkan ke NICE"

status=$(get "$service/devices")
[[ $status == 401 && $(field error_description) == missing_token ]] ||
  fail "GET /devices tanpa token dijawab HTTP $status $(field error_description), seharusnya 401 missing_token."
ok "GET /devices tanpa token: 401 missing_token"

form=$(jq -rn '"grant_type=client_credentials&scope=" + ("nice:aud:nadi" | @uri)
  + "&client_id=" + (env.SMOKE_SA_ID | @uri) + "&client_secret=" + (env.SMOKE_API_KEY | @uri)')
status=$(printf '%s' "$form" | curl -s -o "$body" -w '%{http_code}' --max-time 15 \
  -H 'Content-Type: application/x-www-form-urlencoded' --data-binary @- "$nice/oauth/token" || true)
unset form
if [[ $status != 200 ]]; then
  error=$(field error)
  case $error in
    invalid_client)
      fail "NICE menolak kredensial Service Account: invalid_client." \
        "Periksa checklist langkah 9: SMOKE_SA_ID harus id Service Account (sa_...) pemilik SMOKE_API_KEY," \
        "key belum dicabut atau kedaluwarsa, dan Service Account tidak dinonaktifkan." ;;
    invalid_scope)
      fail "NICE menolak scope nice:aud:nadi: invalid_scope $(field error_description)." \
        "Kemungkinan langkah yang terlewat: 3 (registrasi), 4 (impor), 5 (aktivasi service), 6 (entitlement organisasi)," \
        "7 (aktivasi di project milik Service Account), atau 9 (project Service Account harus mengaktifkan nadi)." ;;
    "")
      fail "permintaan token ke $nice/oauth/token gagal (HTTP $status)." ;;
    *)
      fail "permintaan token ditolak: HTTP $status $error $(field error_description)." ;;
  esac
fi
access_token=$(field access_token)
[[ -n $access_token ]] || fail "jawaban token dari NICE tidak memuat access_token."
ok "token Service Account untuk nice:aud:nadi diterbitkan"

status=$(printf 'header = "Authorization: Bearer %s"\n' "$access_token" |
  curl -s -o "$body" -w '%{http_code}' --max-time 10 -K - "$service/devices" || true)
unset access_token
if [[ $status != 200 ]]; then
  reason=$(field error_description)
  case $reason in
    permission_denied)
      fail "GET /devices dijawab 403 permission_denied: token tidak memuat nadi.devices.read." \
        "Periksa checklist langkah 9: binding Service Account ke role nadi (mis. nadi.viewer) di project-nya." ;;
    wrong_issuer)
      fail "GET /devices dijawab 401 wrong_issuer: NICE_ISSUER di .env berbeda dengan iss token." \
        "Samakan dengan NICE_ISSUER di nice/.env lalu jalankan ulang service (checklist langkah 2)." ;;
    keys_unavailable)
      fail "GET /devices dijawab 503 keys_unavailable: service belum mendapat JWKS dari NICE." \
        "Pastikan service dapat menjangkau $nice/oauth/keys, lalu jalankan ulang service." ;;
    *)
      fail "GET /devices dengan token dijawab HTTP $status $(field error) $reason, seharusnya 200." ;;
  esac
fi
ok "GET /devices dengan token Service Account: 200 ($(jq '.devices | length' "$body") perangkat)"
printf 'smoke nadi lulus\n'
