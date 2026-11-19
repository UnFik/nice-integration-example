# Contoh integrasi NICE

[English](README.en.md)

Lima contoh yang bisa dijalankan untuk cara sistem terhubung dengan NICE. Setiap contoh berdiri sendiri dan punya README, `.env.example`, tes, serta checklist setup NICE masing-masing.

| Contoh | Kapan dipakai | Stack |
|---|---|---|
| [`sharedservice-oauth/`](sharedservice-oauth/README.md) | Service Anda bisa diubah untuk memverifikasi token NICE. Klien memanggilnya langsung, dan NICE tidak berada di jalur request (`auth_mode: nice_token`). UI-nya bicara ke backend service itu saja: backend login ke NICE sebagai service client confidential, memegang token, dan memindah organisasi serta project (backend-for-frontend). Data barang per organisasi dan project, PostgreSQL opsional. | `backend/`: Go + `nice/pkg/nicetoken` + `nice/pkg/niceclient`, port 8081; `web/`: Vite + React, port 5181 saat dev |
| [`shared-service-apikey/`](shared-service-apikey/README.md) | Service lama yang hanya mengenal API key miliknya sendiri. Klien memanggilnya lewat proxy NICE `/<slug>/...` (`auth_mode: api_key`). | TypeScript + Hono, port 8082 |
| [`public-client-pkce/`](public-client-pkce/README.md) | Aplikasi browser milik satu organisasi (client tenant) yang login dengan akun NICE lalu memanggil service `nice_token` atas nama user. | Vite + React + `oauth4webapi`, port 5180 |
| [`agent-backend/`](agent-backend/README.md) | Backend agen AI (MCP server) yang menjalankan tool ke service `nice_token` atas nama user lewat token exchange, tanpa meneruskan token user. Service `api_key` tidak dipanggil: ia memakai kunci per Service Account dan menolak token user. | Go + SDK MCP Go + `nice/pkg/niceclient`, port 8083 |
| [`service-account-client/`](service-account-client/README.md) | Integrasi tanpa user (job, daemon, service mitra) yang memegang API key Service Account, menukarnya ke NICE menjadi token, lalu memanggil service `nice_token` langsung. Key hanya dikirim ke NICE; token dipakai ulang dan diperbarui. | Go + `nice/pkg/niceclient` (`KindServiceAccount`), program baris perintah tanpa port |

```text
 SPA (public client) --PKCE--> NICE /oauth/*
 SPA --Bearer JWT (aud=nadi)--> sharedservice-oauth/backend      (langsung, verifikasi lokal)
 Service Account --Bearer JWT (aud=pengiriman)--> NICE /pengiriman/... --X-Api-Key--> shared-service-apikey
```

```text
 UI --cookie sesi--> sharedservice-oauth/backend --PKCE + client_secret, refresh nice:org nice:aud:nadi nice:prj--> NICE /oauth/token
 sharedservice-oauth/backend --token login ditukar ke nice:aud:nice--> NICE /api/v1/me/contexts
```

```text
 MCP client --Bearer JWT (aud=agen)--> agent-backend --token exchange--> NICE /oauth/token
 agent-backend --Bearer JWT hasil exchange (aud=nadi, act)--> sharedservice-oauth/backend
```

```text
 service-account-client --client_credentials: id Service Account + API key, nice:aud:nadi--> NICE /oauth/token
 service-account-client --Bearer JWT (aud=nadi)--> sharedservice-oauth/backend      (key tidak pernah sampai ke service)
```

SPA tidak memanggil service `api_key` karena path proxy NICE `/<slug>/...` tidak dicakup CORS.

Kode protokol OAuth di browser (`oidc.ts`, token manager) hanya ada di `public-client-pkce`. UI `sharedservice-oauth` tidak memuat kode protokol: semuanya di backend lewat `nice/pkg/niceclient`.

### Versi NICE minimum

| Contoh | Versi NICE |
|---|---|
| `sharedservice-oauth` | `4282316` (`add-client-org-switch`, `add-signin-token-exchange`); jalur Bearer saja cukup `c7a5086` |
| `shared-service-apikey` | `40ddb20` (operasi key wajib pada registrasi `api_key`; mirrored key sampai ke upstream lewat proxy) |
| `public-client-pkce` | `c7a5086`; tanpa `NICE_CORS_ALLOWED_ORIGINS` mulai `60939ed` (`add-delegated-nice-api`) |
| `agent-backend` | `6cc9986` (ditambah `add-token-exchange`); `acted_by` di `sharedservice-oauth` juga butuh revisi ini |
| `service-account-client` | `c7a5086` (sama dengan jalur Bearer `sharedservice-oauth`) |

## Prasyarat

- Susunan direktori: `nice/` harus berada di sebelah `example/`. Contoh Go memakai `nice/pkg/nicetoken` dan `nice/pkg/niceclient` lewat `replace ../../nice` (`agent-backend`, `service-account-client`) dan `replace ../../../nice` (`sharedservice-oauth/backend`).
- NICE dev berjalan di host (`cd nice && make db && make migrate && make dev`, `http://localhost:8080`).
- Pengaturan di `nice/.env` (setelah diubah, jalankan ulang NICE):

  | Variabel | Nilai | Untuk |
  |---|---|---|
  | `NICE_OUTBOUND_ALLOW_HTTP` | `true` | probe health dan proxy ke service `http://127.0.0.1` |
  | `NICE_OUTBOUND_ALLOW_CIDRS` | `127.0.0.0/8` | idem |
  | `NICE_CORS_ALLOWED_ORIGINS` | tambahkan `http://localhost:5180` | SPA, hanya pada NICE sebelum `60939ed`; sesudahnya NICE mengizinkan origin redirect URI client `public` |

- Go 1.27 (contoh Go), Node 22.18+ dan npm (contoh TypeScript), `bash`, `curl`, dan `jq` (skrip smoke).

Daftarkan endpoint service dengan `127.0.0.1`, bukan `localhost`, supaya NICE tidak mencoba `::1`.

Untuk NICE yang berjalan di Docker (`make up` di `nice/`), `docker compose up -d --build` di akar repo ini menjalankan `sharedservice-oauth` (backend dengan UI di `http://localhost:8081`) dan PostgreSQL-nya; daftarkan service sebagai `http://inventaris:8081`. Lihat komentar di `compose.yml`.

## Setup di NICE

Pendaftaran service, impor katalog, entitlement, aktivasi, binding, OAuth client, Service Account, dan API key dilakukan manual. README setiap contoh memuat checklist bernomor berisi langkah di console beserta potongan `curl` setaranya. `scripts/smoke.sh` di contoh service dan `agent-backend` menyebut nomor langkah yang kemungkinan terlewat bila uji gagal.

## Kontrak NICE yang diikuti

- [`nice/docs/token-verification.md`](../nice/docs/token-verification.md): verifikasi token di service, feed perubahan akses, dan operasi sensitif.
- [`nice/docs/gateway-integration.md`](../nice/docs/gateway-integration.md): alur integrasi service dan klien.
- [`nice/docs/api.md`](../nice/docs/api.md): bagian "Token OIDC", "Service api_key dan proxy", "Kunci per Service Account", "Akses lintas origin", "Service client", "Token API NICE terdelegasi", "Pindah organisasi lewat refresh", "Konteks pemanggil", dan "Token exchange: backend atas nama user".
