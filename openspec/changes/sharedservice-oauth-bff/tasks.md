# Tasks

## 1. Kerangka folder

- [x] 1.1 Pastikan `add-client-org-switch` dan `add-signin-token-exchange` sudah ter-merge di `nice` `fikri/dev`; verifikasi: `go doc github.com/nice-platform/nice/pkg/niceclient Client.SignInToken` dari `nice` menampilkan method itu.
- [x] 1.2 `git mv shared-service-oauth sharedservice-oauth/backend` dan `git mv service-client-ui sharedservice-oauth/web`, ganti modul Go menjadi `.../example/sharedservice-oauth/backend` dengan `replace github.com/nice-platform/nice => ../../../nice`, dan sesuaikan `Makefile` dan `Dockerfile`; verifikasi: `make test` di `backend/` dan `npm test` di `web/` lulus sebelum ada perubahan perilaku.

## 2. Login, sesi, dan keluar di backend

- [x] 2.1 Tambah konfigurasi (`NICE_CLIENT_ID`, `NICE_CLIENT_SECRET`, `PUBLIC_URL`, redirect URI turunan, umur sesi) beserta `.env.example` berisi placeholder; verifikasi: tes config menolak start tanpa client ID atau secret dengan pesan yang menyebut nama variabel dan tanpa mencetak nilai.
- [x] 2.2 Tambah penyimpanan sesi di memori (`internal/adapter/memory/session/`) dan use case sesi (`internal/app/session/`) dengan subjek SDK terpisah dari ID sesi; verifikasi: tes use case untuk buat, baca, kedaluwarsa 8 jam, dan hapus.
- [x] 2.3 Tambah `/auth/login`, `/auth/callback`, `/auth/session`, dan `/auth/logout` (`internal/adapter/rest/auth/`) memakai `niceclient` `KindBFF`, userinfo saat callback, dan pemilihan project awal; verifikasi: tes dengan NICE palsu untuk login berhasil, `state` salah, `access_denied`, sesi tanpa cookie (401), `invalid_grant` (401 `session_ended`), keluar berhasil, dan keluar saat NICE mati (sesi tetap); `go test ./internal/... -count=1` dan `go test ./internal/ -run Arch` lulus.

## 3. Middleware sesi dan CSRF

- [x] 3.1 Tambah middleware yang mengisi `Authorization: Bearer` dari token konteks aktif untuk request ber-cookie tanpa `Authorization`, sebelum rantai `nicetoken` yang ada; verifikasi: tes izin (403 `permission_denied`), tenant setelah pindah konteks, dan jalur Bearer langsung (`agent-backend`, Service Account) yang tidak berubah.
- [x] 3.2 Tambah pemeriksaan CSRF (`X-CSRF-Token` dan `Origin` sama dengan `PUBLIC_URL`) untuk request tulis ber-cookie, cookie `inventaris_session` dengan atribut sesuai skema; verifikasi: tes form lintas origin (403, data tidak berubah), header token salah (403), dan pemanggil Bearer tanpa cookie (diproses).

## 4. Konteks dan pindah konteks

- [x] 4.1 Tambah `GET /auth/contexts` (token login, exchange ke `nice`, semua halaman `/me/contexts`); verifikasi: tes untuk A/P, A/Q, B/R, organisasi tanpa project, dan penolakan exchange.
- [x] 4.2 Tambah `POST /auth/context` (refresh dengan `Key.Org`, ganti konteks hanya bila berhasil, 409 dengan alasan untuk `access_denied`/`invalid_scope`); verifikasi: tes pindah organisasi (satu refresh dengan `nice:org`), akses dicabut (konteks tetap, refresh token tidak berubah), dan dua request serentak dari satu sesi dengan `-race`.

## 5. Data barang per org dan project

- [x] 5.1 Tambah `DATABASE_URL` (opsional) ke konfigurasi dan `.env.example`, migrasi SQL `tenants` dan `items` (dengan `down`), serta layanan `postgres` di `compose.yml`; verifikasi: tes config (kosong memilih memori, terisi memilih PostgreSQL, nilai tidak pernah dicetak).
- [x] 5.2 Ubah domain, use case, dan adapter memori agar repository menerima kunci scope `{OrgID, ProjectID, Subject, Actor}` dari klaim token dan menolak token tanpa `prj` dengan 403 `project_required`; verifikasi: tes use case dan handler (project P tidak terlihat dari Q, `project_id` dari query atau body diabaikan, token tanpa `prj` ditolak) dan `go test ./internal/ -run Arch` lulus.
- [x] 5.3 Tambah `internal/adapter/postgres/item/` yang mengimplementasikan repository yang sama dengan filter `org_id` dan `project_id` di setiap query, provisioning tenant `ON CONFLICT DO NOTHING`, `created_by` dari `sub` dan `acted_by` dari `act`, lalu pilih adapter di `bootstrap` menurut `DATABASE_URL`; verifikasi: tes repository terhadap PostgreSQL tes untuk org A/B dan project P/Q, id dari scope lain menjadi 404, dan dua request pertama serentak dengan `-race`.
- [x] 5.4 Periksa bahwa token Service Account (`scripts/smoke.sh`) dan `agent-backend` membawa `prj`; bila tidak, sesuaikan spec dan skrip; verifikasi: smoke menunjukkan 200 untuk `GET /items` dengan token Service Account.

## 6. Satu origin

- [x] 6.1 Tambah penyaji `web/dist` di backend dan proxy Vite untuk `/auth` dan `/items` di `web/vite.config.ts`, lalu hapus `http://localhost:5181` dari CORS bawaan; verifikasi: tes backend `GET /` menyajikan `index.html` bila `dist` ada, dan `npm run build` lalu `make run` menyajikan UI di 8081.

## 7. Web hanya UI

- [x] 7.1 Hapus `oidc.ts`, `token-manager.ts`, `contexts.ts`, `jwt.ts`, `storage.ts`, `recent-contexts.ts`, `scope.ts`, `ManualContextForm`, `TokensSection`, NICE palsu di `src/test/`, dan dependensi `oauth4webapi`; tambah klien `api.ts` untuk `/auth/*` dan `/items*` dengan cookie dan `X-CSRF-Token`; verifikasi: `npm run typecheck` lulus dan `grep -r oauth4webapi web/` kosong.
- [x] 7.2 Sesuaikan `session.ts` dan komponen (`Home`, `ContextPicker`, `ItemsSection`, `Screens`) agar membaca `/auth/session` dan `/auth/contexts`, menampilkan tombol dari `perms`, dan menangani 401, 409, serta galat keluar; verifikasi: Vitest dengan fetch stub untuk masuk, pemilih, pindah konteks, akses dicabut, sesi berakhir, dan tidak adanya token di `localStorage`/`sessionStorage`; `npm test` dan `npm run build` lulus.

## 8. Rujukan di seluruh repo

- [x] 8.1 Perbarui `compose.yml` (konteks build `sharedservice-oauth/backend`, env `PUBLIC_URL` 8081), `scripts/smoke.sh` (pindah ke `sharedservice-oauth/backend/scripts/`, tambah cek `/auth/session` 401 dan `/auth/login` mengarah ke NICE), README akar (`README.md`, `README.en.md`), `AGENTS.md` (tabel contoh, perintah, aturan duplikasi kode protokol SPA kini hanya `public-client-pkce`), serta rujukan di `agent-backend` dan `public-client-pkce`; verifikasi: `grep -rn "shared-service-oauth\|service-client-ui" --exclude-dir=node_modules --exclude-dir=archive .` kosong, dan `docker compose config` valid.

## 9. Dokumentasi dan uji manual

- [x] 9.1 Tulis `sharedservice-oauth/README.md` dan `README.en.md` (isi folder, cara kerja BFF, konfigurasi, checklist setup NICE: client confidential di project organisasi platform diikat ke `inventaris`, dua redirect URI, dua organisasi yang memakai `inventaris`; checklist uji browser per requirement; catatan Store produksi, tabel data per org dan project, dan bahwa token memberi scope sedangkan nama org dan project dibaca dari NICE); verifikasi: kedua README setara isinya dan setiap langkah menyebut hasil yang diharapkan.
- [ ] 9.2 Jalankan checklist terhadap NICE dev, `scripts/smoke.sh`, dan isi baris "Diuji terhadap nice revisi"; verifikasi: semua langkah uji sesuai hasil yang diharapkan dan smoke keluar dengan kode 0.

## Workflow follow-up

- Kerjakan di worktree `.worktrees/example-sharedservice-oauth-bff` dari `main`, branch `feat/sharedservice-oauth-bff`. `openspec/` di `example` ter-track, jadi pindahkan `openspec/changes/sharedservice-oauth-bff/` ke worktree.
- Setelah semua tugas tercentang dan tes terkait hijau, arsipkan change ini, commit, merge ke `main`, lalu hapus worktree dan branch-nya.
