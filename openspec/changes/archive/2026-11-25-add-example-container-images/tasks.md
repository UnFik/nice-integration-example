# Tasks

## 1. shared-service-apikey

- [x] 1.1 Tambahkan `shared-service-apikey/Dockerfile` (node 22 alpine, `npm ci --omit=dev`, pengguna `node`, `LISTEN_ADDR=0.0.0.0:8082`, `KEYS_FILE=/data/keys.json`, `VOLUME /data`, `EXPOSE 8082`, `HEALTHCHECK` ke `/healthz`) dan `.dockerignore` (`node_modules`, `.env`, `data`)
- [x] 1.2 README (id dan en): bagian "Menjalankan di container" (service tetap privat, volume data harus bertahan) dan role `pengiriman.operator` di langkah 5
- [x] 1.3 Verifikasi: `heavy npm test -- test/`, `heavy npm run typecheck`, dan `heavy docker build -t pengiriman:check shared-service-apikey`

## 2. public-client-pkce: konfigurasi nadi

- [x] 2.1 `.env.example` `VITE_SERVICES` dan contoh di pesan galat `src/config.ts` memakai `nadi` dengan `/devices`
- [x] 2.2 README (id dan en): prasyarat, CORS, checklist setup dan uji memakai `nadi`/`/devices`; klaim penolakan API NICE diperhalus (token penawaran `nice:aud:nice`, `/me/contexts` tetap menolak); hapus batas "tidak ada kontainer"
- [x] 2.3 Pastikan tampilan body `{"devices":[...]}` di panel service masuk akal (JSON dirapikan, tanpa perubahan kode)

## 3. public-client-pkce: image

- [x] 3.1 Tambahkan `Dockerfile` (build arg untuk setiap `VITE_*`, `npm ci && npm run build`, runtime `nginxinc/nginx-unprivileged:1.30-alpine`), `nginx.conf` (fallback SPA, `index.html` `no-cache`, `/assets/` cache panjang), dan `.dockerignore`
- [x] 3.2 README (id dan en): bagian membangun image (build arg, issuer = `NICE_ISSUER`, endpoint service `https` publik, redirect `https://<host>/callback` dan post-logout `https://<host>/`)
- [x] 3.3 Verifikasi: `heavy npm test -- src/`, `heavy npm run typecheck`, `heavy docker build` dengan build arg, lalu container: `/` dan `/callback` dijawab 200 dengan `index.html`

## 4. Versi

- [x] 4.1 Baris "Diuji terhadap nice revisi" `public-client-pkce` (id dan en): tes unit terhadap `nice` `dev` `eb17848` (2026-11-25), uji manual browser belum diulang
- [x] 4.2 README akar (id dan en): hanya baris `shared-service-apikey` dan `public-client-pkce` di tabel versi
