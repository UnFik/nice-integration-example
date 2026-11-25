# Proposal

## Why

`shared-service-apikey` dan `public-client-pkce` akan dijalankan di VPS kecil di belakang Cloudflare Tunnel, dengan NICE di issuer `https`. Kedua contoh belum punya image container, padahal `sharedservice-oauth` sudah punya (`backend/Dockerfile`, `web/Dockerfile` dengan nginx). Tanpa image, mitra harus merakit sendiri cara menjalankan Node dengan type stripping dan cara menyajikan SPA dengan fallback `/callback`.

Selain itu konfigurasi `public-client-pkce` basi: `.env.example`, pesan galat `VITE_SERVICES`, dan README masih menunjuk service `inventaris` di `/items`, padahal backend contoh sekarang slug `nadi` dengan `/devices`. README juga menyatakan token client non-sistem ditolak API NICE, padahal client tenant kini bisa memperoleh token penawaran `nice:aud:nice` (hanya `/me/contexts` yang tetap menolaknya). README `shared-service-apikey` langkah 5 tidak menyebut role `pengiriman.operator` yang diturunkan NICE sejak `b65a3e3`. Baris "Diuji terhadap nice revisi" `public-client-pkce` menyebut `c7a5086`, yang tidak ada lagi di riwayat `nice`.

## What Changes

- `shared-service-apikey/Dockerfile` dan `.dockerignore`: image Node 22 alpine, hanya dependensi runtime, pengguna non-root, `LISTEN_ADDR=0.0.0.0:8082`, `KEYS_FILE=/data/keys.json` di volume `/data`, `HEALTHCHECK` ke `/healthz`. README (id dan en) mendapat bagian "Menjalankan di container": service harus tetap privat (hanya proxy NICE yang menjangkaunya) dan volume data harus bertahan.
- README `shared-service-apikey` langkah 5: pratinjau impor juga menampilkan role `pengiriman.operator`.
- `public-client-pkce`: `.env.example`, contoh di pesan galat `VITE_SERVICES`, checklist uji, prasyarat, dan bagian CORS memakai `nadi` dengan `/devices`. Klaim "token client non-sistem ditolak API NICE" diperhalus.
- `public-client-pkce/Dockerfile`, `nginx.conf`, dan `.dockerignore`: build stage dengan build arg untuk setiap `VITE_*`, runtime `nginxinc/nginx-unprivileged:1.30-alpine` dengan fallback SPA, `index.html` tanpa cache, aset ber-hash dengan cache panjang. README (id dan en) mendapat bagian membangun image.
- Baris versi README `public-client-pkce` dan baris `public-client-pkce` serta `shared-service-apikey` di tabel versi README akar.

Bukan **BREAKING**: perilaku service dan SPA tidak berubah.

## Capabilities

### New Capabilities

Tidak ada.

### Modified Capabilities

- `api-key-service-example`: image container dengan penyimpanan key di volume.
- `public-client-example`: image container dengan konfigurasi saat build; contoh konfigurasi memakai `nadi`; alasan daftar service dari konfigurasi diperhalus.

## Impact

- Berkas baru: `shared-service-apikey/{Dockerfile,.dockerignore}`, `public-client-pkce/{Dockerfile,nginx.conf,.dockerignore}`.
- Berkas berubah: `public-client-pkce/{.env.example,src/config.ts,README.md,README.en.md}`, `shared-service-apikey/{README.md,README.en.md}`, dua baris tabel versi di `README.md` dan `README.en.md` akar.
- Tidak ada perubahan di repo `nice`.
