# Design

## Context

Image `sharedservice-oauth` menjadi acuan: `backend/Dockerfile` (build bertahap, runtime non-root, data di `/data`) dan `web/Dockerfile` (Node build, `nginxinc/nginx-unprivileged:1.30-alpine`, fallback SPA, `index.html` tanpa cache). Target deployment kedua contoh adalah VPS kecil di belakang Cloudflare Tunnel, dengan NICE di issuer `https`.

`shared-service-apikey` dijalankan langsung dari `.ts` lewat type stripping Node 22.18+, tanpa build. Dependensi runtime-nya (`hono`, `@hono/node-server`, `@hono/zod-openapi`, `zod`) ada di `dependencies`; `typescript`, `vitest`, dan `@types/node` hanya `devDependencies`. Berkas key ditulis atomik (berkas sementara di direktori yang sama lalu `rename`), jadi direktori `KEYS_FILE` harus bisa ditulis pengguna proses.

`public-client-pkce` membaca konfigurasi dari `import.meta.env` (`VITE_NICE_ISSUER`, `VITE_NICE_CLIENT_ID`, `VITE_REDIRECT_URI`, `VITE_POST_LOGOUT_REDIRECT_URI`, `VITE_SERVICES`), yang dibakukan Vite saat build. SPA menampilkan body jawaban service sebagai JSON yang dirapikan, jadi jawaban `nadi` `{"devices":[...]}` tampil tanpa perubahan kode.

## Goals / Non-Goals

**Goals:**

- Image yang bisa langsung dijalankan untuk kedua contoh, dengan pola yang sama dengan `sharedservice-oauth`.
- Konfigurasi dan dokumentasi `public-client-pkce` menunjuk backend contoh yang ada (`nadi`, `/devices`).

**Non-Goals:**

- Konfigurasi SPA saat runtime (mis. `config.js` yang diisi entrypoint). Contoh tetap memakai `VITE_*` saat build.
- Compose atau konfigurasi Cloudflare Tunnel. `compose.yml` akar tidak diubah.
- TLS di dalam container: TLS diakhiri di tunnel.

## Decisions

1. **`shared-service-apikey`: `node:22-alpine`, satu stage.** `npm ci --omit=dev` cukup karena tidak ada build. Proses dijalankan `node src/index.ts` sebagai pengguna `node` (bukan `npm start`, supaya sinyal sampai ke proses Node dan `.env` tidak dibaca di container). Bawaan image: `LISTEN_ADDR=0.0.0.0:8082` (di container loopback tidak terjangkau dari luar) dan `KEYS_FILE=/data/keys.json`, dengan `/data` dibuat milik `node` dan dideklarasikan `VOLUME`. `HEALTHCHECK` memakai `wget` busybox ke `http://127.0.0.1:8082/healthz`. Alternatif distroless ditolak: tidak ada `wget` untuk healthcheck, dan keuntungannya kecil untuk contoh.
2. **`public-client-pkce`: build arg per `VITE_*`.** Setiap arg diteruskan sebagai `ENV` di stage build; Vite membaca variabel `VITE_*` dari lingkungan proses. Nilai kosong sama dengan tidak diisi (bawaan `<origin>/callback` dan `<origin>/` untuk URI). `.env` dikecualikan `.dockerignore`, jadi build tidak diam-diam memakai konfigurasi dev. Alternatif konfigurasi runtime ditolak (lihat Non-Goals): mengubah `src/config.ts` dan menambah jalur yang tidak dipakai saat dev.
3. **nginx tanpa template.** SPA tidak punya backend yang diteruskan, jadi `nginx.conf` disalin apa adanya ke `/etc/nginx/conf.d/default.conf`. `index.html` `no-cache`; `/assets/` (nama ber-hash dari Vite) `max-age=31536000, immutable`; path berekstensi lain `try_files $uri =404`; selebihnya, termasuk `/callback`, jatuh ke `index.html`.
4. **Versi NICE.** `c7a5086` dan `60939ed` hilang dari riwayat `nice` setelah riwayat disusun ulang; commit tertua sekarang `5def119` (2026-11-17), yang sudah memuat `add-delegated-nice-api`. Baris versi minimum `public-client-pkce` di README akar menunjuk `5def119`. Baris "Diuji terhadap nice revisi" menyebut tes unit terhadap `nice` `dev` `eb17848` (2026-11-25) dan bahwa uji manual browser belum diulang.

## Risks / Trade-offs

- [Konfigurasi dibakukan saat build] → satu image per lingkungan. README menjelaskannya dan menyebut build arg yang dibutuhkan.
- [Volume `/data` hilang] → semua mirrored key ditolak 401 dan NICE harus membuat ulang kunci. README menegaskan volume harus bertahan.
- [Service `api_key` terbuka ke publik] → siapa pun yang memegang `SERVICE_CREDENTIAL` bisa mengaku tenant mana pun. README menyatakan container tidak boleh dipublikasikan lewat tunnel; hanya NICE yang menjangkaunya.
- Uji manual browser terhadap NICE `https` belum dijalankan dalam change ini; hanya tes unit dan build image.
