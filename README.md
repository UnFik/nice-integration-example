# Contoh integrasi NICE

[English](README.en.md)

Empat contoh yang bisa dijalankan untuk cara sistem terhubung dengan NICE. Setiap contoh berdiri sendiri dan punya README, `.env.example`, tes, serta checklist setup NICE masing-masing.

| Contoh | Kapan dipakai | Stack |
|---|---|---|
| [`shared-service-oauth/`](shared-service-oauth/README.md) | Service Anda bisa diubah untuk memverifikasi token NICE. Klien memanggilnya langsung, dan NICE tidak berada di jalur request (`auth_mode: nice_token`). | Go + `nice/pkg/nicetoken`, port 8081 |
| [`shared-service-apikey/`](shared-service-apikey/README.md) | Service lama yang hanya mengenal API key miliknya sendiri. Klien memanggilnya lewat proxy NICE `/<slug>/...` (`auth_mode: api_key`). | TypeScript + Hono, port 8082 |
| [`public-client-pkce/`](public-client-pkce/README.md) | Aplikasi browser milik satu organisasi (client tenant) yang login dengan akun NICE lalu memanggil service `nice_token` atas nama user. | Vite + React + `oauth4webapi`, port 5180 |
| [`service-client-ui/`](service-client-ui/README.md) | UI service first-party yang melayani banyak organisasi (service client): konteks dari `/api/v1/me/contexts`, pindah organisasi dan project lewat refresh. | Vite + React + `oauth4webapi`, port 5181 |

```text
 SPA (public client) --PKCE--> NICE /oauth/*
 SPA --Bearer JWT (aud=inventaris)--> shared-service-oauth      (langsung, verifikasi lokal)
 Service Account --Bearer JWT (aud=pengiriman)--> NICE /pengiriman/... --X-Api-Key--> shared-service-apikey
```

```text
 UI (service client) --PKCE, lalu refresh nice:aud:nice--> NICE /api/v1/me/contexts
 UI --refresh nice:org nice:aud:inventaris nice:prj--> NICE /oauth/token --> UI --Bearer JWT--> shared-service-oauth
```

SPA tidak memanggil service `api_key` karena path proxy NICE `/<slug>/...` tidak dicakup CORS.

Kode protokol SPA (`oidc.ts`, token manager, NICE palsu untuk tes) sengaja diduplikasi di `public-client-pkce` dan `service-client-ui`, supaya setiap contoh bisa disalin sendiri. Perbaikan protokol di salah satunya perlu diterapkan ke keduanya.

### Versi NICE minimum

| Contoh | Versi NICE |
|---|---|
| `shared-service-oauth` | `c7a5086` |
| `shared-service-apikey` | `2638605` (mirrored key sampai ke upstream lewat proxy) |
| `public-client-pkce` | `c7a5086`; tanpa `NICE_CORS_ALLOWED_ORIGINS` mulai `60939ed` (`add-delegated-nice-api`) |
| `service-client-ui` | `8160ac4` (`add-service-clients`, `add-delegated-nice-api`, `add-seamless-org-switch`) |

## Prasyarat

- Susunan direktori: `nice/` harus berada di sebelah `example/`. Contoh Go memakai `nice/pkg/nicetoken` lewat `replace ../../nice`.
- NICE dev berjalan di host (`cd nice && make db && make migrate && make dev`, `http://localhost:8080`).
- Pengaturan di `nice/.env` (setelah diubah, jalankan ulang NICE):

  | Variabel | Nilai | Untuk |
  |---|---|---|
  | `NICE_OUTBOUND_ALLOW_HTTP` | `true` | probe health dan proxy ke service `http://127.0.0.1` |
  | `NICE_OUTBOUND_ALLOW_CIDRS` | `127.0.0.0/8` | idem |
  | `NICE_CORS_ALLOWED_ORIGINS` | tambahkan `http://localhost:5180` | SPA, hanya pada NICE sebelum `60939ed`; sesudahnya NICE mengizinkan origin redirect URI client `public` |

- Go 1.27 (contoh Go), Node 22.18+ dan npm (contoh TypeScript), `bash`, `curl`, dan `jq` (skrip smoke).

Daftarkan endpoint service dengan `127.0.0.1`, bukan `localhost`, supaya NICE tidak mencoba `::1`.

## Setup di NICE

Pendaftaran service, impor katalog, entitlement, aktivasi, binding, OAuth client, Service Account, dan API key dilakukan manual. README setiap contoh memuat checklist bernomor berisi langkah di console beserta potongan `curl` setaranya. `scripts/smoke.sh` di kedua contoh service menyebut nomor langkah yang kemungkinan terlewat bila uji gagal.

## Kontrak NICE yang diikuti

- [`nice/docs/token-verification.md`](../nice/docs/token-verification.md): verifikasi token di service, feed perubahan akses, dan operasi sensitif.
- [`nice/docs/gateway-integration.md`](../nice/docs/gateway-integration.md): alur integrasi service dan klien.
- [`nice/docs/api.md`](../nice/docs/api.md): bagian "Token OIDC", "Service api_key dan proxy", "Kunci per Service Account", "Akses lintas origin", "Service client", "Token API NICE terdelegasi", "Pindah organisasi lewat refresh", dan "Konteks pemanggil".
