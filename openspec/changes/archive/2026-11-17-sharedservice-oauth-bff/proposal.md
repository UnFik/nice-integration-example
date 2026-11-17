# Proposal

## Why

Contoh UI service (`service-client-ui`) dan service `inventaris` (`shared-service-oauth`) sekarang dibagi menurut asumsi SPA: browser memegang refresh token, bicara langsung ke NICE (login PKCE, refresh, pindah organisasi, `/me/contexts`), lalu memanggil backend service dengan Bearer. Akibatnya:

- Logika protokol (~630 baris TypeScript: `oidc.ts`, `token-manager.ts`, `contexts.ts`) hidup di browser dan harus diduplikasi dengan `public-client-pkce`.
- Refresh token berotasi dipegang di `sessionStorage`, sehingga tab kedua mematikan keluarga token.
- Penemuan konteks lewat refresh `nice:aud:nice` kini ditolak NICE untuk service client (`invalid_scope`), jadi UI jatuh ke form konteks manual.
- Mitra yang membangun service nyata biasanya punya backend. Pola yang mereka butuhkan adalah backend yang menangani OAuth ke NICE, dan UI yang hanya bicara ke backend itu.

NICE sudah mendukung pola itu: service client confidential boleh login, refresh, dan pindah organisasi dengan `client_secret`, dan `pkg/niceclient` punya `KindBFF`, `AuthorizeURL`, `HandleCallback`, `Disconnect`, serta `Store` dengan kunci per sesi. Dua kekurangan ditutup oleh change di repo `nice` yang harus ter-merge lebih dulu:

- `add-client-org-switch`: pindah organisasi lewat refresh di `niceclient`.
- `add-signin-token-exchange`: penemuan konteks sejak login. Token exchange menerima token login client sendiri untuk tujuan `nice`, karena tanpa itu service client tidak bisa membaca `/me/contexts` sebelum project diketahui.

## What Changes

- **Folder baru `sharedservice-oauth/`** menggantikan `shared-service-oauth/` dan `service-client-ui/`, berisi `backend/` (Go) dan `web/` (React + Vite).
- **Backend** (dasarnya `shared-service-oauth`) menambah:
  - `/auth/login`, `/auth/callback`, `/auth/logout`: authorization code + PKCE sebagai service client **confidential** lewat `niceclient` (`KindBFF`), dengan `client_secret` hanya di env backend.
  - Session server-side: cookie `HttpOnly` berisi session ID acak, konteks aktif `{org, project}`, dan token CSRF. Refresh token ada di `Store` SDK, tidak pernah sampai ke browser.
  - `GET /auth/session` (identitas dan konteks aktif), `GET /auth/contexts` (pilihan organisasi dan project dari `/api/v1/me/contexts`, dengan token login sesi yang ditukar ke `nice`), dan `POST /auth/context` (pindah org/project lewat refresh `nice:org`/`nice:aud`/`nice:prj`).
  - Middleware session: untuk request ber-cookie, mengisi `Authorization: Bearer` dari token konteks aktif, lalu rantai `nicetoken` dan handler barang yang sudah ada berjalan tanpa diubah. Jalur Bearer langsung tetap hidup untuk `agent-backend`, klien Service Account, dan `public-client-pkce`.
- **Frontend** (`web/`) hanya UI: tombol masuk, pemilih konteks, tabel dan dialog barang, status sesi. Tanpa `oidc.ts`, token manager, atau `contexts.ts`. Semua panggilan ke `/auth/*` dan `/items*` di origin yang sama, dengan cookie dan header CSRF.
- **Data barang per org dan project di PostgreSQL (opsional):** `DATABASE_URL` memilih adapter PostgreSQL di samping adapter memori; tanpa itu tetap memori. Tabel `tenants(org_id)` dan `items` berkunci `org_id` (dari `nice_org_id`), `project_id` (dari `prj`), dan `created_by` (dari `sub`). Repository selalu memfilter dengan klaim token, bukan input klien. Barang kini milik project aktif, jadi `GET /items` hanya memuat barang project itu, dan token tanpa `prj` dijawab 403 `project_required`. Nama org dan project tidak disimpan; label dibaca dari NICE.
- **Satu origin:** dev memakai proxy Vite ke backend, produksi menyajikan `web/dist` dari backend.
- **Dihapus:** `shared-service-oauth/` dan `service-client-ui/` beserta spec `service-client-ui-example`.
- **Diperbarui:** `compose.yml`, `README.md`/`README.en.md` akar, `AGENTS.md`, `scripts/smoke.sh`, dan rujukan di `agent-backend` serta `public-client-pkce`. Pencarian rujukan harus mencakup kedua ejaan lama (`shared-service-oauth`) dan baru (`sharedservice-oauth`), yang hanya beda satu tanda hubung.
- **Checklist setup NICE** berubah: client service bertipe confidential (secret masuk `.env` backend), redirect URI mengarah ke callback backend, dan langkah dependensi `nice` serta form konteks manual dihapus.

Di luar cakupan:
- Store refresh token berbasis database atau Redis (contoh memakai `MemoryStore` atau `FileStore`; README menjelaskan kebutuhan produksi). Database hanya dipakai untuk data barang;
- akun lokal dan login selain NICE;
- mengakhiri sesi SSO NICE saat logout (hanya refresh token dicabut);
- perubahan di `agent-backend`, `shared-service-apikey`, dan `public-client-pkce` selain rujukan yang berubah.

## Capabilities

### New Capabilities
- `service-bff-example`: contoh service fullstack yang backend-nya menangani OAuth ke NICE (login, callback, session, refresh, pindah organisasi dan project, discovery konteks, logout) dan UI-nya hanya bicara ke backend itu.

### Modified Capabilities
- `nice-token-service-example`: konfigurasi (client confidential, redirect URI, pengaturan session, `DATABASE_URL`), panduan setup/uji, serta operasi barang dan data per org kini juga per project (PostgreSQL opsional) berubah; verifikasi token, operasi sensitif, feed, JWKS, OpenAPI, health, CORS, dan `acted_by` tetap.
- `service-client-ui-example`: seluruh requirement dihapus; perilakunya pindah ke `service-bff-example`.

## Impact

- Kode: folder baru `sharedservice-oauth/`; penghapusan `shared-service-oauth/` dan `service-client-ui/`; perubahan di `compose.yml`, README, `AGENTS.md`, dan `scripts/smoke.sh`.
- Data: migrasi SQL dan layanan `postgres` di `compose.yml`; `GET /items` dan operasi barang lain kini berscope org dan project (perubahan perilaku untuk klien yang selama ini melihat semua barang org).
- Dependensi: `sharedservice-oauth/backend` memakai `github.com/nice-platform/nice` lewat `replace ../../../nice` (satu level lebih dalam dari sekarang) dan butuh `niceclient` hasil `add-client-org-switch` dan `add-signin-token-exchange`. `web/` memakai React 19, Vite, dan Vitest; `oauth4webapi` dihapus.
- NICE: tidak berubah oleh change ini. Perubahan NICE ada di dua change prasyarat.
- Urutan: `add-client-org-switch`, lalu `add-signin-token-exchange` (repo `nice`) di-merge lebih dulu, baru change ini di-apply.
