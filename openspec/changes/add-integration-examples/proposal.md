# Proposal

## Why

Mitra yang mengintegrasikan sistemnya dengan NICE sudah punya panduan di `nice-docs` (memanggil service, melindungi service, service lama, proxy API key), tetapi belum ada kode yang bisa dijalankan. Akibatnya, bagian yang paling sering salah harus ditebak dari prosa: verifikasi token, feed perubahan akses, operasi sensitif, kredensial service di balik proxy, dan token per service lewat refresh token yang berotasi. Tim NICE juga belum punya sandbox dev untuk menguji ketiga jalur integrasi itu secara end-to-end.

## What Changes

- Repo `example/` berisi tiga contoh lepas. Setiap contoh berdiri sendiri, dengan README, `.env.example`, tes, dan skrip uji masing-masing:
  - `shared-service-oauth/`: SharedService bermode `nice_token` (slug `inventaris`, Go + `pkg/nicetoken`). Klien memanggilnya langsung dengan JWT NICE, dan service memverifikasi sendiri: izin per operasi, tenant per org, JWKS di disk, feed perubahan akses, dan pemeriksaan online untuk operasi sensitif.
  - `shared-service-apikey/`: SharedService bermode `api_key` (slug `pengiriman`, TypeScript + Hono). Service lama yang hanya mengenal API key miliknya sendiri dan dipanggil lewat proxy NICE. Contoh ini juga menyediakan tiga operasi key upstream untuk mode mirrored key.
  - `public-client-pkce/`: SPA React yang menjadi public client tenant (authorization code + PKCE). SPA menampilkan identitas user, lalu memanggil satu atau lebih service `nice_token` dengan token per service yang diambil lewat refresh token.
- Semua langkah di sisi NICE (registrasi service, impor katalog, entitlement, aktivasi, binding, OAuth client, Service Account, API key) dilakukan manual oleh pengguna. README tiap contoh memuat checklist console beserta potongan `curl` setaranya.
- Berkas meta repo: README indeks, `AGENTS.md`, `CLAUDE.md`, `.gitignore`. Akar `CLAUDE.md` workspace mencatat repo ketiga ini.

Di luar cakupan: perubahan pada NICE (termasuk CORS untuk path proxy `/<slug>/...`, sehingga SPA tidak memanggil service `api_key`), tautan dari `nice-docs`, docker compose, dan skrip yang mendaftarkan apa pun ke NICE.

## Capabilities

### New Capabilities
- `nice-token-service-example`: contoh SharedService `nice_token` yang memverifikasi token NICE sendiri dan menegakkan izin, tenant, pencabutan cepat, dan operasi sensitif sesuai kontrak verifikasi token NICE.
- `api-key-service-example`: contoh SharedService `api_key` di balik proxy NICE yang memeriksa kredensial miliknya sendiri, membaca identitas dari header proxy, dan melayani operasi key upstream untuk mirrored key.
- `public-client-example`: contoh SPA public client yang login lewat PKCE dan mengelola token per service dengan refresh token yang berotasi.

### Modified Capabilities
(tidak ada; repo baru)

## Impact

- Repo baru `example/` (git, branch `main`). Tidak ada kode NICE atau console yang berubah.
- Ketergantungan: contoh Go memakai modul `github.com/nice-platform/nice` lewat `replace` ke `../../nice` (repo saudara di workspace), Go 1.27, dan swag v1 sebagai `tool`. Contoh TypeScript memakai Node 22.18+, npm, Hono, `@hono/zod-openapi`, Vite, React 19, `oauth4webapi`, dan Vitest.
- Menjalankan contoh menuntut NICE dev (`make dev`) yang diatur dengan `NICE_OUTBOUND_ALLOW_HTTP=true`, `NICE_OUTBOUND_ALLOW_CIDRS=127.0.0.0/8`, dan origin `http://localhost:5180` di `NICE_CORS_ALLOWED_ORIGINS`. README mendokumentasikannya; contoh tidak mengubah `nice/.env`.
- Port tetap: 8081 (`inventaris`), 8082 (`pengiriman`), 5180 (SPA).
- Contoh mengikuti kontrak NICE saat ini (`docs/token-verification.md`, `docs/api.md`, `docs/gateway-integration.md`). Perubahan kontrak di NICE dapat membuat contoh tertinggal; skrip smoke tiap contoh menjadi alat deteksinya.
