# sharedservice-oauth (`nadi`)

[English](README.en.md)

Contoh SharedService fullstack untuk NICE dengan slug `nadi` (NADI: CRUD perangkat IoT). Backend Go adalah service `nice_token` yang memverifikasi token NICE sendiri, sekaligus backend-for-frontend (BFF) UI-nya: ia login ke NICE sebagai OAuth client **confidential** yang diikat ke `nadi`, menyimpan refresh token di server, dan memberi browser cookie sesi saja. UI React hanya berbicara ke backend di origin yang sama: tidak memegang token, tidak memanggil NICE, dan tidak memuat pustaka OAuth.

Diuji terhadap nice revisi: SDK `v0.2.2` dan NICE `dev` `1b137fa` (2026-11-24): tes unit, dan menu akun diuji manual di browser. **Ganti akun** menampilkan pemilih akun NICE, lalu masuk sebagai akun lain. **Keluar** mencabut refresh token, mengakhiri sesi NICE lewat `/oauth/end_session`, kembali ke `PUBLIC_URL/`, dan login berikutnya meminta password. SDK `v0.2.1` dan NICE `dev` `d42b398` (2026-11-24): tes unit, dan login lewat UI, pemilih konteks dari `ExchangeClient.Contexts` dengan organisasi act-as operator platform, lalu pindah ke project organisasi lain, diuji manual. SDK `v0.2.0` (2026-11-24): tes unit, dan jalur `perms_ref` diuji manual (lihat [`backend/README.md`](backend/README.md)). SDK `v0.1.0` dan server NICE lokal di Docker (image `nice:dev` dari `c7a3e33`), 2026-11-19: image `nadi` dibangun ulang tanpa konteks `nice`, tes unit dan `backend/scripts/smoke.sh` lulus. Alur UI di bawah tidak diuji ulang di revisi ini. Terhadap `4282316` (NICE dev dibangun dari revisi itu): login, callback, penemuan konteks, pindah org dan project, isolasi data per org dan project (memori dan PostgreSQL), CSRF, keluar, dan `scripts/smoke.sh` lulus lewat HTTP. Langkah klik di browser belum diuji manual.

Versi NICE minimum: `b361d25` (`service-calls-without-service-account`; sebelumnya `4282316`, `add-client-org-switch` dan `add-signin-token-exchange`). SDK: `github.com/ABA-Developer/nice-auth-sdk` `v0.2.3`. Service tidak lagi memegang kredensial NICE selain OAuth client UI: feed perubahan akses, pemeriksaan online, dan penyelesaian `perms_ref` berjalan tanpa konfigurasi.

## Isi

| Folder | Isi |
|---|---|
| [`backend/`](backend/README.md) | Service `nadi` (Go, `nice-auth-sdk/verify`) untuk pemanggil Bearer, ditambah `/auth/*` untuk UI (`nice-auth-sdk/credentials`, `KindBFF`), penyaji `web/dist`, dan penyimpanan perangkat per organisasi dan project (memori atau PostgreSQL). README-nya memuat operasi, checklist setup service, dan smoke |
| [`web/`](web/README.md) | UI React 19 + Vite: masuk, pemilih konteks, dan tabel perangkat. Hanya memanggil `/auth/*` dan `/devices*`. `Dockerfile`-nya membangun image nginx yang menyajikan hasil build dan meneruskan `/auth` serta `/devices` ke backend |

## Cara kerja

```text
 browser --GET /auth/login--> backend --302--> NICE /oauth/authorize (PKCE S256, state terikat sesi)
 NICE --302 /auth/callback?code--> backend --code + client_secret--> NICE /oauth/token
 browser --cookie nadi_session--> backend /auth/session, /auth/contexts, /auth/context, /auth/logout
 browser --cookie + X-CSRF-Token--> backend /devices --Bearer token konteks aktif--> rantai niceauth/verify --> handler perangkat
 agent/backend, Service Account --Bearer--> backend /devices   (jalur langsung, cookie tidak dipakai)
```

- **Login.** `GET /auth/login` membuat sesi tertunda (cookie berumur 10 menit), lalu mengalihkan ke NICE dengan scope `openid profile email offline_access`. `GET /auth/callback` mencocokkan `state` dengan sesi browser itu, menukar `code` dengan `client_secret`, lalu membuat sesi baru dengan ID baru. Callback dengan `state` asing, sudah dipakai, atau milik browser lain ditolak tanpa menyimpan token. Galat login (`access_denied`, `state_mismatch`, `temporarily_unavailable`) dikirim ke UI sebagai `/?login_error=<kode>`; URL tidak pernah memuat token.
- **Sesi.** Cookie `nadi_session` berisi ID acak 32 byte (`HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` bila `PUBLIC_URL` berskema `https`). Rekaman sesi di server menyimpan subjek `niceauth/credentials` acak lain, identitas dari `/oauth/userinfo`, konteks aktif `{org, project}`, dan token CSRF. Refresh token disimpan `credentials.MemoryStore` per subjek, jadi nilai cookie tidak pernah menjadi kunci token. Sesi berakhir setelah `SESSION_IDLE_TIMEOUT` (bawaan 15 hari) tidak dipakai, dan backend mencabut refresh token-nya.
- **Konteks.** Setelah login dan setiap `GET /auth/contexts`, backend mengambil token login (`Client.SignInToken`, refresh dengan scope dasar saja), menukarnya ke token API NICE terdelegasi (`ExchangeClient`, `nice:aud:nice`), lalu membaca semua halaman `GET /api/v1/me/contexts`. Setelah login, project pertama organisasi saat ini langsung dipilih. Sesi hanya menyimpan ID; nama organisasi dan project selalu dibaca dari NICE.
- **Pindah konteks.** `POST /auth/context {org, project}` meminta token dengan `Key{Org, Audience: "nadi", Project}`: satu refresh dengan `nice:org:<org> nice:aud:nadi nice:prj:<project>` ditambah scope dasar. Konteks sesi baru diganti bila NICE menerbitkan token. Penolakan (`access_denied`, `invalid_scope`) dijawab `409 context_refused`, konteks tetap, dan refresh token tidak berubah. Dua request serentak dari satu sesi memakai refresh token satu per satu.
- **Request perangkat dari UI.** Untuk request ber-cookie tanpa `Authorization`, middleware sesi memasang token konteks aktif sebagai `Authorization: Bearer`, lalu rantai `niceauth/verify` yang sama dengan pemanggil Bearer berjalan: verifikasi, izin, tenant dari `nice_org_id`, feed, dan pemeriksaan online. Cache `niceauth/credentials` tidak tahu isi feed. Karena itu, bila feed sudah menandai token konteks aktif basi (mis. role pengguna diubah), middleware membuang token itu dan meminta token baru dari NICE sekali sebelum meneruskan request; `perms` di `GET /auth/session` memakai jalur yang sama. Bila token baru pun basi, request dijawab `401 token_stale` tanpa permintaan lagi. Request yang membawa `Authorization` diproses sebagai Bearer dan cookie diabaikan; pemanggil Bearer memperbarui tokennya sendiri.
- **CSRF.** Request tulis ber-cookie (`POST`, `PUT`, `DELETE`, termasuk `/auth/context` dan `/auth/logout`) wajib membawa `X-CSRF-Token` dari `GET /auth/session` dan `Origin` sama dengan `PUBLIC_URL`; selain itu `403 {"error":"csrf"}`.
- **Keluar dan ganti akun.** Menu akun di pojok kanan atas punya dua pilihan. Keduanya mulai dengan `POST /auth/logout`, yang mencabut refresh token di NICE (`/oauth/revoke`), menghapus sesi dan cookie, lalu menjawab `end_session_url`. Bila NICE tidak menjawab, jawabannya `503` dan sesi tetap, supaya pengguna tidak tampak keluar saat token masih berlaku.
  - **Keluar** lalu membawa browser ke `end_session_url` (`/oauth/end_session` NICE, dibangun `Client.EndSessionURL` dengan ID token login sebagai `id_token_hint`). NICE mengakhiri sesi SSO-nya dan mengembalikan browser ke `POST_LOGOUT_URL`, jadi **Masuk** berikutnya meminta password.
  - **Ganti akun** membawa browser ke `/auth/login?prompt=select_account`. Sesi SSO NICE tetap, dan NICE menampilkan pemilih akun alih-alih langsung memasukkan akun yang sama.
- **Data perangkat.** Perangkat milik satu organisasi dan satu project: kunci scope `{OrgID, ProjectID, Subject, Actor}` diambil dari klaim token terverifikasi (`nice_org_id`, `prj`, `sub`, `act`), tidak pernah dari request. Token tanpa `prj` dijawab `403 project_required`. Lihat [`backend/README.md`](backend/README.md#data-per-organisasi-dan-project).

### Jawaban `/auth/*`

| Operasi | Jawaban |
|---|---|
| `GET /auth/login[?prompt=select_account]` | `302` ke NICE |
| `GET /auth/callback` | `302` ke `PUBLIC_URL/`, atau `PUBLIC_URL/?login_error=<kode>` |
| `GET /auth/session` | `200 {"user":{"id","name","email"},"context":{"org","project"}\|null,"perms":[...],"csrf_token":"...","context_error":""}`; `401 unauthenticated` atau `401 session_ended` |
| `GET /auth/contexts` | `200 {"current_organization_id","organizations":[{"id","name","slug","projects":[{"id","name","slug"}],"projects_truncated"}]}`; `502 contexts_unavailable` (deskripsi berisi kode dari NICE); `503` |
| `POST /auth/context` | `200` seperti `/auth/session`; `400 invalid_argument`; `403 csrf`; `409 context_refused`; `503` |
| `POST /auth/logout` | `200 {"end_session_url": ...}`, juga tanpa sesi; `403 csrf`; `503 temporarily_unavailable` (sesi tetap) |

Request perangkat ber-cookie juga bisa dijawab `401 unauthenticated`/`session_ended`, `409 context_required` (belum ada konteks), atau `409 context_refused`. Operasi `/auth/*` tidak masuk dokumen OpenAPI, karena bukan operasi katalog NICE.

## Konfigurasi

Semua konfigurasi ada di `backend/.env` (salin dari `backend/.env.example`). UI tidak punya konfigurasi; image Docker-nya hanya membaca `BACKEND_ADDR`, alamat backend (bawaan `nadi:8081`).

| Variabel | Bawaan | Arti |
|---|---|---|
| `NICE_ISSUER` | wajib | issuer NICE, sama persis dengan klaim `iss` |
| `NICE_CLIENT_ID` | wajib | `client_id` OAuth client confidential yang diikat ke `nadi` |
| `NICE_CLIENT_SECRET` | wajib | secret client itu. Hanya dikirim ke NICE; tidak pernah dicetak |
| `PUBLIC_URL` | `http://localhost:8081` | origin yang dilihat browser. Dev dengan Vite dan Docker: `http://localhost:5181`. Redirect URI diturunkan sebagai `PUBLIC_URL/auth/callback` |
| `POST_LOGOUT_URL` | `PUBLIC_URL/` | tujuan browser setelah **Keluar** mengakhiri sesi NICE. Harus terdaftar sebagai post-logout redirect URI client; `none` membiarkan browser di halaman keluar NICE |
| `SESSION_IDLE_TIMEOUT` | `360h` | sesi yang tidak dipakai selama ini berakhir (`360h` berarti 15 hari) |
| `WEB_DIR` | `../web/dist` | hasil `npm run build` yang disajikan di `PUBLIC_URL`; tanpa direktori itu backend tidak menyajikan UI. Docker mengosongkannya, karena UI punya container sendiri |
| `DATABASE_URL` | kosong | PostgreSQL untuk perangkat; kosong berarti memori |
| `LISTEN_ADDR`, `JWKS_FILE`, `CORS_ALLOWED_ORIGINS`, `SMOKE_SA_ID`, `SMOKE_API_KEY` | | lihat [`backend/README.md`](backend/README.md#konfigurasi) |

Tanpa `NICE_ISSUER`, `NICE_CLIENT_ID`, atau `NICE_CLIENT_SECRET`, backend berhenti saat start dengan pesan yang menyebut nama variabelnya.

## Menjalankan

Pengembangan (UI di `http://localhost:5181`, `PUBLIC_URL=http://localhost:5181` di `backend/.env`):

```bash
cd web && npm install && cd ..  # pasang dependensi sekali
make run                        # backend di 8081 dan web di 5181
```

Tekan Ctrl+C untuk menghentikan keduanya. Vite meneruskan `/auth` dan `/devices` ke backend.

Build, satu proses (UI di `http://localhost:8081`, `PUBLIC_URL=http://localhost:8081`):

```bash
cd web && npm run build         # web/dist
cd backend && make run          # menyajikan web/dist di 8081
```

Docker: `make up` di folder ini (setara `docker compose up -d --build` di akar repo) menjalankan backend, UI, dan PostgreSQL untuk `DATABASE_URL` dalam container terpisah (lihat `compose.yml`):

- backend (`backend/Dockerfile`) di `http://localhost:8081` untuk pemanggil Bearer, tanpa UI;
- UI (`web/Dockerfile`, nginx) di `http://localhost:5181`, yang meneruskan `/auth` dan `/devices` ke backend seperti proxy Vite, jadi browser tetap melihat satu origin.

`backend/.env` tetap dibutuhkan untuk `NICE_ISSUER`, `NICE_CLIENT_ID`, dan `NICE_CLIENT_SECRET`; `PUBLIC_URL`, `WEB_DIR`, dan `DATABASE_URL` diisi `compose.yml`. Container UI dan `npm run dev` sama-sama memakai port 5181, jadi jalankan salah satunya saja.

Tes: `make test` di `backend/` (tanpa NICE; NICE palsu `httptest`), dan `npm test` di `web/` (backend palsu lewat stub `fetch`).

## Checklist setup NICE

1. Jalankan langkah 1-10 [`backend/README.md`](backend/README.md#checklist-setup-nice). Langkah 2 membuat OAuth client confidential di project organisasi platform dengan dua redirect URI (`http://localhost:5181/auth/callback` untuk dev dan Docker, `http://localhost:8081/auth/callback` untuk build satu proses), dan smoke di langkah 10 memeriksa `/auth/session` serta `/auth/login`.
2. Ikat client ke `nadi` (butuh `nice.platform.catalog.manage`). Ikatan menjadikan client itu service client: login memilih organisasi user yang meng-entitle `nadi`, dan client boleh pindah organisasi lewat refresh.

   ```bash
   curl -sS "$NICE/api/v1/projects/$UI_PROJECT_ID/clients/$OAUTH_CLIENT_ID/service" -X PUT \
     -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"nadi"}' \
     | jq '.oauth_client | {client_class, service}'
   # {"client_class":"service","service":"nadi"}
   ```

3. Siapkan dua organisasi yang memakai `nadi`: jalankan langkah 6-8 backend untuk organisasi A dengan project P dan Q, dan organisasi B dengan project R, dengan user uji yang sama sebagai anggota keduanya dan binding `nadi.editor` di P dan R. Untuk Q, beri binding `nadi.viewer` saja (uji izin baca). Opsional: organisasi C milik user uji yang di-entitle `nadi` tanpa binding di project mana pun.
4. Tidak ada yang perlu diubah di `nice/.env` untuk UI: browser hanya bernavigasi ke halaman login NICE, tidak memanggil API NICE, jadi CORS NICE tidak terlibat.

Hasil yang diharapkan: `GET /api/v1/projects/$UI_PROJECT_ID/clients/$OAUTH_CLIENT_ID/token-reach` menjawab `client_class` `service`, `service` `nadi`, `base_scopes` memuat `offline_access`, dan `may_switch_organization` `true`.

## Checklist uji browser

Buka DevTools (tab Network dengan **Preserve log**, dan Application > Storage). A, B, C, P, Q, dan R dari langkah 3 di atas. Jalankan backend dan UI seperti di "Menjalankan" (dev, build, atau Docker), dan log NICE (`make dev`) di terminal lain.

| # | Requirement | Langkah | Hasil yang diharapkan |
|---|---|---|---|
| 1 | Login lewat backend | Tekan **Masuk**, login sebagai user uji | Navigasi ke `/auth/login`, lalu ke `$NICE/oauth/authorize` dengan `code_challenge_method=S256` dan `scope=openid profile email offline_access`. Kembali ke `/` tanpa `code`, `state`, atau token di URL. Cookie `nadi_session` ber-`HttpOnly` dan `SameSite=Lax`. Nama dan email tampil, konteks aktif project pertama organisasi saat ini, dan perangkatnya tampil |
| 2 | Tidak ada organisasi yang memakai service | Login dengan akun tanpa organisasi ber-entitlement `nadi` | Layar menjelaskan bahwa tidak ada organisasi pengguna yang memakai service ini, dengan tombol **Masuk dengan akun lain** (`/auth/login?prompt=select_account`). Tidak ada cookie sesi |
| 3 | Callback tidak cocok | Buka `/auth/callback?code=x&state=palsu` | Kembali ke `/?login_error=state_mismatch`, tidak ada sesi, dan log NICE tidak menunjukkan `POST /oauth/token` |
| 4 | Sesi sisi server | Setelah langkah 1, periksa Network dan Storage | Panel jaringan hanya menunjukkan request ke origin UI (`/auth/*`, `/devices`) selain navigasi login. `localStorage`, `sessionStorage`, dan jawaban `/auth/session` tidak memuat access token, refresh token, atau ID token (tidak ada string `eyJ`) |
| 5 | Konteks dari NICE | Buka pemilih konteks | `GET /auth/contexts` memuat tepat A/P, A/Q, dan B/R dengan nama dari NICE; konteks aktif ditandai. C tampil tidak tersedia dengan alasan pengguna tidak punya akses `nadi` di project mana pun. Log NICE menunjukkan token exchange dan `GET /api/v1/me/contexts` |
| 6 | Pindah organisasi | Dari A/P, pilih B/R | Satu `POST /auth/context`; log NICE menunjukkan tepat satu refresh dengan `nice:org:<B> nice:aud:nadi nice:prj:<R>` dan audit `token.org_switch`. Halaman tidak berpindah; perangkat B tampil dan perangkat A tidak |
| 7 | Izin dari token | Pilih A/Q (hanya `nadi.viewer`) | Tombol Tambah, Ubah, dan Hapus tidak tersedia; daftar perangkat Q tampil (kosong, terpisah dari P) |
| 8 | Akses dicabut di antara | Buka pemilih, lalu nonaktifkan `nadi` di Q (console: project Q, **Product & Activation**), lalu pilih Q | `POST /auth/context` dijawab `409 context_refused` (`invalid_scope`), konteks aktif tetap, UI menampilkan alasannya dan memuat ulang pilihan; Q tidak lagi ditawarkan. Aktifkan lagi Q sesudahnya |
| 9 | Kelola perangkat | Di A/P, tambah, ubah, lalu hapus perangkat | `POST /devices`, `PUT /devices/<id>`, `DELETE /devices/<id>` membawa `X-CSRF-Token` dan dijawab sukses (hapus butuh `nadi.admin` dan diperiksa online ke NICE dengan token konteks aktif). `created_by` perangkat adalah `usr_` user uji |
| 10 | CSRF | Dari konsol DevTools di halaman lain (mis. `http://localhost:5180`), kirim form `POST` ke `/devices` di origin UI | Dijawab `403 csrf`; perangkat tidak bertambah |
| 11 | Pemanggil Bearer | Jalankan `backend/scripts/smoke.sh` saat sesi browser masih ada | Smoke lulus; `GET /devices` dengan token Service Account dijawab `200` tanpa terpengaruh cookie |
| 12 | Sesi NICE berakhir | Akhiri sesi user di NICE (console: akun, **Sesi**, cabut sesi ini), lalu pilih konteks yang belum pernah dipakai | `401 session_ended`; UI menampilkan sesi berakhir dan tombol **Masuk** |
| 13 | Restart backend | Login, hentikan dan jalankan ulang backend, muat ulang halaman | `GET /auth/session` dijawab `401`; UI menampilkan **Masuk** |
| 14 | Keluar | Buka menu akun, pilih **Keluar** | `POST /auth/logout` dijawab `200`; log NICE menunjukkan `POST /oauth/revoke` lalu `GET /oauth/end_session`; browser kembali ke `PUBLIC_URL/` dan menampilkan **Masuk**. **Masuk** berikutnya meminta password |
| 15 | Keluar saat NICE mati | Login, hentikan NICE, pilih **Keluar** | `503`; UI tetap masuk dan menawarkan mencoba lagi. Setelah NICE berjalan lagi, **Coba lagi** berhasil |
| 16 | Satu origin | Dev: panggilan `/auth/session` dan `/devices` berasal dari `http://localhost:5181` tanpa preflight `OPTIONS`. Build: buka `http://localhost:8081`. Docker: buka `http://localhost:5181` | `GET /` menyajikan UI dan `GET /auth/session` dijawab backend di origin yang sama |
| 17 | Ganti akun | Buka menu akun, pilih **Ganti akun** | `POST /auth/logout` dijawab `200`, lalu NICE menampilkan pemilih akun tanpa meminta password untuk akun yang sudah ada; memilih akun lain masuk sebagai akun itu |

## Catatan produksi

- **Store refresh token.** Contoh ini memakai `credentials.MemoryStore` dan sesi di memori: restart berarti semua orang masuk lagi, dan hanya satu proses yang bisa berjalan. Produksi menyimpan refresh token di database atau Redis yang terenkripsi, mengimplementasikan `credentials.Store` termasuk `Lock` lintas proses (refresh token berotasi dan hanya boleh dipakai satu proses sekaligus), dan menyimpan sesi di tempat yang dibagi semua proses. Store harus memisahkan token per `Key.Org`.
- **Cookie.** Cookie `localhost` terlihat oleh semua port, jadi nama cookie dibuat khas. Produksi memakai domain sendiri, HTTPS (`PUBLIC_URL` `https://...` menyalakan `Secure`), dan redirect URI `https`.
- **Data per organisasi dan project.** Tabel `tenants` dan `devices` berkunci `org_id` dan `project_id` dari token, tanpa foreign key ke NICE. Row-level security PostgreSQL (`SET LOCAL app.org_id`) bisa ditambahkan sebagai pengaman kedua. Organisasi atau project yang dihapus di NICE tidak memberi tahu service; datanya tetap sampai service membersihkannya sendiri.
- **Label.** Token hanya memberi scope (ID). Nama organisasi dan project dibaca dari NICE (`/auth/contexts`) setiap kali pemilih dibuka, karena bisa berubah di NICE.
- **Keluar.** Access token yang sudah terbit tetap berlaku sampai kedaluwarsa (TTL access token NICE); yang dihentikan adalah refresh token dan sesi SSO. ID token disimpan di sesi server hanya untuk `id_token_hint`, tidak pernah dikirim ke browser.
