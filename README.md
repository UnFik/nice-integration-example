# Contoh integrasi NICE

[English](README.en.md)

Tiga contoh yang bisa dijalankan untuk tiga cara sistem terhubung dengan NICE. Setiap contoh berdiri sendiri dan punya README, `.env.example`, tes, serta checklist setup NICE masing-masing.

| Contoh | Kapan dipakai | Stack |
|---|---|---|
| [`shared-service-oauth/`](shared-service-oauth/README.md) | Service Anda bisa diubah untuk memverifikasi token NICE. Klien memanggilnya langsung, dan NICE tidak berada di jalur request (`auth_mode: nice_token`). | Go + `nice/pkg/nicetoken`, port 8081 |
| [`shared-service-apikey/`](shared-service-apikey/README.md) | Service lama yang hanya mengenal API key miliknya sendiri. Klien memanggilnya lewat proxy NICE `/<slug>/...` (`auth_mode: api_key`). | TypeScript + Hono, port 8082 |
| [`public-client-pkce/`](public-client-pkce/README.md) | Aplikasi browser yang login dengan akun NICE lalu memanggil service `nice_token` atas nama user. | Vite + React + `oauth4webapi`, port 5180 |

```text
 SPA (public client) --PKCE--> NICE /oauth/*
 SPA --Bearer JWT (aud=inventaris)--> shared-service-oauth      (langsung, verifikasi lokal)
 Service Account --Bearer JWT (aud=pengiriman)--> NICE /pengiriman/... --X-Api-Key--> shared-service-apikey
```

SPA tidak memanggil service `api_key` karena path proxy NICE `/<slug>/...` tidak dicakup CORS.

## Prasyarat

- Susunan direktori: `nice/` harus berada di sebelah `example/`. Contoh Go memakai `nice/pkg/nicetoken` lewat `replace ../../nice`.
- NICE dev berjalan di host (`cd nice && make db && make migrate && make dev`, `http://localhost:8080`).
- Pengaturan di `nice/.env` (setelah diubah, jalankan ulang NICE):

  | Variabel | Nilai | Untuk |
  |---|---|---|
  | `NICE_OUTBOUND_ALLOW_HTTP` | `true` | probe health dan proxy ke service `http://127.0.0.1` |
  | `NICE_OUTBOUND_ALLOW_CIDRS` | `127.0.0.0/8` | idem |
  | `NICE_CORS_ALLOWED_ORIGINS` | tambahkan `http://localhost:5180` | SPA |

- Go 1.27 (contoh Go), Node 22.18+ dan npm (contoh TypeScript), `bash`, `curl`, dan `jq` (skrip smoke).

Daftarkan endpoint service dengan `127.0.0.1`, bukan `localhost`, supaya NICE tidak mencoba `::1`.

## Setup di NICE

Pendaftaran service, impor katalog, entitlement, aktivasi, binding, OAuth client, Service Account, dan API key dilakukan manual. README setiap contoh memuat checklist bernomor berisi langkah di console beserta potongan `curl` setaranya. `scripts/smoke.sh` di kedua contoh service menyebut nomor langkah yang kemungkinan terlewat bila uji gagal.

## Kontrak NICE yang diikuti

- [`nice/docs/token-verification.md`](../nice/docs/token-verification.md): verifikasi token di service, feed perubahan akses, dan operasi sensitif.
- [`nice/docs/gateway-integration.md`](../nice/docs/gateway-integration.md): alur integrasi service dan klien.
- [`nice/docs/api.md`](../nice/docs/api.md): bagian "Token OIDC", "Service api_key dan proxy", "Kunci per Service Account", dan "Akses lintas origin".
