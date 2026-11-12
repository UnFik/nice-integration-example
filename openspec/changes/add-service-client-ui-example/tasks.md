# Tasks

## 1. Kerangka `service-client-ui`

- [x] 1.1 Buat proyek Vite + React 19 + TypeScript di `service-client-ui/` (port 5181 `strictPort`, `oauth4webapi`, Vitest, skrip `dev`/`build`/`test`/`typecheck`, `.env.example` dengan issuer, `client_id`, redirect URI, post-logout URI, dan endpoint `inventaris`); verifikasi `npm run typecheck` dan `npm run build` lulus

## 2. Protokol dan token

- [x] 2.1 Salin dan sesuaikan modul OIDC (PKCE, callback, `access_denied` dengan pesan service tidak tersedia, issuer `http` hanya loopback) dari `public-client-pkce`; verifikasi tes callback termasuk `access_denied`
- [x] 2.2 Implementasikan token manager berkunci `(org, service, project)` dengan scope dasar di setiap refresh, `nice:org` hanya saat organisasi berubah, organisasi aktif dari klaim token terakhir, antrean refresh tunggal, pakai ulang token, dan satu pengulangan pada `token_stale`; verifikasi tes untuk pindah organisasi, kembali ke konteks lama tanpa permintaan token baru, dan penolakan tanpa rotasi
- [x] 2.3 Perluas NICE palsu: `nice:org` saat refresh, `/api/v1/me/contexts` dengan token terdelegasi, `invalid_scope` untuk project tidak aktif; verifikasi tes membuktikan kode akan gagal bila `nice:org` atau scope dasar dihilangkan

## 3. Layar

- [x] 3.1 Implementasikan identitas (token client lalu userinfo) dan pemilih konteks dari `/me/contexts` (organisasi tanpa project ditandai tidak tersedia, konteks aktif ditandai); verifikasi tes komponen bahwa pemilih hanya berisi konteks dari jawaban
- [x] 3.2 Implementasikan pindah konteks dengan satu refresh, tetap di konteks lama dan muat ulang konteks saat ditolak, dan daftar barang `inventaris` di konteks aktif; verifikasi tes alur penuh dengan NICE palsu dan `inventaris` palsu

## 4. Dokumentasi contoh baru

- [ ] 4.1 Tulis `README.md` dan `README.en.md`: versi NICE minimum, checklist setup manual (client `public` di project organisasi platform diikat ke `inventaris`, dependensi `inventaris` → `nice` dengan `nice.me.contexts.read`, dua organisasi ber-entitlement), cara menjalankan, dan checklist uji manual per requirement; verifikasi nama variabel sama dengan `.env.example` dan tautan relatif valid

## 5. Penyesuaian contoh lain

- [ ] 5.1 `shared-service-oauth`: tambah `http://localhost:5181` ke `CORS_ALLOWED_ORIGINS` di `.env.example` dan README; verifikasi tes konfigurasi `.env.example` tetap lulus
- [ ] 5.2 `public-client-pkce`: README `id` dan `en` menjelaskan client tenant lawan service client (tautan ke `service-client-ui`) dan bahwa `NICE_CORS_ALLOWED_ORIGINS` tidak wajib pada NICE dengan `add-delegated-nice-api`; verifikasi tautan relatif valid
- [ ] 5.3 README indeks `id` dan `en` serta `AGENTS.md`: contoh baru, port 5181, perintah, versi NICE minimum per contoh, dan catatan bahwa kode protokol SPA diduplikasi di dua contoh; verifikasi tautan relatif valid

## 6. Verifikasi integrasi terhadap NICE dev

Grup ini butuh NICE dev dengan `add-service-clients`, `add-delegated-nice-api`, dan `add-seamless-org-switch`, serta setup manual dari README.

- [ ] 6.1 Jalankan `shared-service-oauth` dan `service-client-ui`, lalu jalankan checklist uji manual di browser (login di organisasi ber-entitlement pertama, pemilih dari `/me/contexts`, pindah organisasi tanpa redirect, penolakan saat akses dicabut, reload tetap login); verifikasi setiap langkah memberi hasil yang diharapkan dan catat revisi `nice` di README

## Workflow follow-up

- Mulai setelah ketiga change NICE yang dibutuhkan digabung ke `main` repo `nice`.
- Setelah tugas selesai dan uji lulus, arsipkan change ini dan commit di `main` repo `example`.
