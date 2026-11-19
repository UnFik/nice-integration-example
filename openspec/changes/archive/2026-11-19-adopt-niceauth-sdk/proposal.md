# Proposal

## Why

Change `rename-sdk-to-niceauth` di repo `nice` memindah SDK Go ke `pkg/niceauth/credentials` (dulu `pkg/niceclient`) dan `pkg/niceauth/verify` (dulu `pkg/nicetoken`) dan menghapus nama lama tanpa alias. Tiga contoh Go di repo ini mengimpor nama lama lewat `replace` ke checkout `nice`, jadi begitu change itu masuk, `sharedservice-oauth/backend`, `agent-backend`, dan `service-account-client` berhenti ter-build. `sharedservice-oauth/backend` yang paling terdampak: ia memakai kedua paket sekaligus, di 21 berkas Go.

Change itu juga menambah `Client.HTTPClient` dan `NewFromEnv`, yang membuat integrasi Service Account jauh lebih pendek. Contoh adalah tempat mitra pertama kali melihat SDK, jadi contoh harus memperlihatkan bentuk yang baru.

## What Changes

- `sharedservice-oauth/backend`: impor dan nama paket mengikuti `niceauth/verify` (verifier, middleware, feed, pemeriksaan online) dan `niceauth/credentials` (BFF: login, refresh, pindah konteks, exchange). Tidak ada perubahan perilaku, rute, izin, atau dokumen OpenAPI.
- `agent-backend`: impor dan nama paket mengikuti `niceauth/verify` dan `niceauth/credentials` (mode exchange). Tidak ada perubahan perilaku.
- `service-account-client`: impor mengikuti `niceauth/credentials`, dan gateway `nadi` memakai `*http.Client` dari `Client.HTTPClient` sebagai pengganti `Client.Do`. README memperlihatkan bentuk tersingkat dengan `NewFromEnv`.
- Dokumen: README dua bahasa setiap contoh Go, README akar dua bahasa, dan `AGENTS.md` menyebut nama paket baru; teks log yang dikutip (`niceclient: ...`, `nicetoken: ...`) menjadi `niceauth: ...`; versi NICE minimum ketiga contoh Go naik ke revisi `nice` yang memuat `rename-sdk-to-niceauth`; baris "Diuji terhadap nice revisi" diperbarui setelah uji ulang.
- **BREAKING** bagi yang menyalin contoh: contoh Go tidak lagi ter-build terhadap `nice` sebelum revisi itu.

`shared-service-apikey` dan `public-client-pkce` (TypeScript) tidak memakai SDK Go dan tidak berubah, kecuali dua kalimat README `public-client-pkce` yang menyebut nama paket.

## Capabilities

### New Capabilities

Tidak ada.

### Modified Capabilities

- `agent-backend-example`: dua requirement menyebut jalur paket SDK (`pkg/nicetoken`, `pkg/niceclient`) dan mengikuti nama baru. Perilakunya tetap.

Spec `nice-token-service-example`, `service-bff-example`, dan `service-account-client-example` tidak menyebut nama paket dan perilakunya tidak berubah, jadi tidak ada delta untuk ketiganya.

## Impact

- **Kode:** 21 berkas Go di `sharedservice-oauth/backend`, 9 di `agent-backend`, dan berkas `adapter/nadi`, `bootstrap`, serta tesnya di `service-account-client`. Tes arsitektur ketiga contoh melarang impor dengan awalan `github.com/nice-platform/nice/`, jadi aturannya tetap berlaku tanpa diubah.
- **Build Docker:** `sharedservice-oauth/backend/Dockerfile` menyalin seluruh `pkg/` dari konteks `nice`, jadi tidak perlu diubah, tetapi image `nadi` harus dibangun ulang.
- **Dokumen:** 10 berkas Markdown yang menyebut nama lama (README contoh, README akar, `AGENTS.md`).
- **Prasyarat:** `rename-sdk-to-niceauth` sudah di-merge di `nice` dan checkout `nice` di sebelah `example/` berada pada revisi itu. Sebelum itu change ini tidak bisa di-build; sesudah itu sampai change ini selesai, contoh Go tidak ter-build.
- **Tidak terdampak:** kontrak NICE di kabel, setup NICE (service, client, Service Account, binding), `.env`, dan `compose.yml`.
