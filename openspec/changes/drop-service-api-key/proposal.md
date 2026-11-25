## Why

NICE change `service-calls-without-service-account` menghapus role `nice.service` beserta API key-nya: feed perubahan akses menjadi publik dan memuat kunci tersamar (`key`, `org_key`), sedangkan pemeriksaan online dan penyelesaian `perms_ref` diautentikasi dengan token request yang diteruskan service. `nice-auth-sdk` versi pasangannya (rilis sesudah `v0.2.2`, belum di-tag) menghapus `FeedConfig.APIKey` dan argumen API key `NewHTTPChecker`/`NewHTTPResolver`. Contoh `nadi` dan `agen` masih mewajibkan `NICE_SERVICE_API_KEY` untuk fitur itu dan tidak lagi bisa dibangun terhadap SDK baru.

## What Changes

- **BREAKING** (contoh): `NICE_SERVICE_API_KEY` dihapus dari `sharedservice-oauth/backend` dan `agent-backend` (config, `.env.example`, wiring verifier, README). Feed, pemeriksaan online `DELETE /devices/{id}`, dan penyelesaian `perms_ref` selalu menyala tanpa konfigurasi.
- Wiring `nadi`: `verify.NewHTTPChecker(issuer)`, `verify.NewHTTPResolver(issuer)`, dan `StartFeed` dengan `verify.FeedConfig{}`; peringatan "tanpa key" saat start hilang. Wiring `agen`: `verify.NewHTTPResolver(issuer)` selalu dipasang.
- NICE palsu di tes `nadi` mengikuti protokol baru: feed tanpa `Authorization` dengan `key`/`org_key`, pemeriksaan (`{"permission": ...}`) dan penyelesaian (`{}`) diotorisasi token Bearer yang diteruskan, dan `401` dari NICE membuat service menjawab `503`.
- Checklist README `nadi`: langkah 9 "Service Account platform `nice.service`" dihapus, langkah 10 dan 11 menjadi 9 dan 10. Rujukan nomor langkah di `scripts/smoke.sh`, `sharedservice-oauth/README*.md`, `public-client-pkce/README*.md`, dan `service-account-client/README*.md` ikut diperbarui.
- Baris versi: NICE minimum menjadi NICE dengan change `service-calls-without-service-account`, SDK rilis sesudah `v0.2.2` (belum di-tag), di README contoh dan tabel versi README akar (`sharedservice-oauth`, `agent-backend`, `service-account-client`).
- `service-account-client` tidak berubah kodenya: API key-nya adalah kredensial Service Account miliknya sendiri. Hanya teks yang menyebut `NICE_SERVICE_API_KEY` di `nadi` diperbarui.
- `go.mod` tetap `v0.2.2` sampai SDK di-tag; sampai saat itu modul dibangun dengan `go.work` lokal yang tidak di-commit.

## Capabilities

### New Capabilities

(tidak ada)

### Modified Capabilities

- `nice-token-service-example`: pemeriksaan online, feed, dan penyelesaian `perms_ref` tanpa API key `nice.service`; konfigurasi tanpa key itu.
- `agent-backend-example`: penyelesaian `perms_ref` selalu aktif dengan token yang diteruskan.

## Impact

- `sharedservice-oauth/backend`: `internal/platform/config`, `internal/bootstrap` (`verifier.go`, `bootstrap.go`, tes wiring, BFF stale, helper), `.env.example`, README id/en, `scripts/smoke.sh`.
- `agent-backend`: `internal/platform/config`, `internal/bootstrap` (`bootstrap.go`, `verifier_test.go`), `.env.example`, README id/en.
- Dokumen: README akar, `sharedservice-oauth/README*.md`, `public-client-pkce/README*.md`, `service-account-client/README*.md`.
- Dependensi: butuh `github.com/ABA-Developer/nice-auth-sdk` rilis sesudah `v0.2.2` dan NICE dengan change `service-calls-without-service-account`. Contoh ini tidak jalan terhadap NICE lama (feed lama butuh API key) maupun SDK `v0.2.2`.
