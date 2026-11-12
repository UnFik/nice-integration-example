# service-client-ui

[English](README.en.md)

UI React untuk service `inventaris` (`../shared-service-oauth`) yang masuk ke NICE sebagai **service client**: OAuth client `public` di project organisasi platform yang diikat ke `inventaris`. UI melayani user di setiap organisasinya yang memakai `inventaris`:

- organisasi dan project yang bisa dipilih dibaca dari `GET /api/v1/me/contexts`, dengan token API NICE terdelegasi (`aud=nice`);
- pindah organisasi dan project memakai satu refresh token (`nice:org`, `nice:aud`, `nice:prj`), tanpa redirect ke NICE;
- daftar barang `inventaris` ditampilkan di konteks yang dipilih.

Kapan memakai contoh ini dan kapan [`../public-client-pkce`](../public-client-pkce/README.md): client tenant milik satu organisasi dan satu project memakai `public-client-pkce`. UI service first-party yang melayani banyak organisasi memakai contoh ini.

Versi NICE minimum: revisi yang memuat change `add-service-clients`, `add-delegated-nice-api`, dan `add-seamless-org-switch` (`nice` `8160ac4` atau sesudahnya).

Diuji terhadap nice revisi: belum diuji terhadap NICE yang berjalan

## Isi

| Berkas | Isi |
| --- | --- |
| `src/scope.ts` | Scope dasar (`BASE_SCOPE`) dan `scopeFor`: scope dasar + `nice:org` (hanya bila organisasi berubah) + `nice:aud` + `nice:prj` |
| `src/oidc.ts` | Discovery, login, callback (termasuk `access_denied`), refresh, userinfo, revoke, dan end-session dengan `oauth4webapi` |
| `src/token-manager.ts` | Token per `(organisasi, service, project)`: organisasi sesi dari klaim token terakhir, antrean satu refresh, pakai ulang, backoff, sesi berakhir, ulang sekali pada token basi. Tanpa React |
| `src/contexts.ts` | `GET /api/v1/me/contexts` dengan token terdelegasi, semua halaman |
| `src/session.ts` | Perekat: login, muat ulang, identitas, konteks, pindah konteks, daftar barang, logout |
| `src/config.ts` | Validasi `VITE_*` |
| `src/ui/`, `src/App.tsx` | Halaman. `App` memeriksa `location.pathname === "/callback"`, tanpa router |

Kode protokol (`oidc.ts`, token manager, NICE palsu di `src/test/`) disalin dari `public-client-pkce` lalu disesuaikan, supaya contoh ini bisa disalin sendiri. Perbaikan protokol di salah satunya perlu diterapkan ke keduanya.

## Prasyarat

- Node 22.18+ dan npm.
- NICE dev berjalan (`cd nice && make dev`, bawaannya `http://localhost:8080`). Untuk NICE di alamat lain, pakai alamat itu di `NICE` di bawah dan di `VITE_NICE_ISSUER`.
- `inventaris` terdaftar, diimpor, dan aktif: langkah 1-5 di [`../shared-service-oauth/README.md`](../shared-service-oauth/README.md). Jalankan servicenya (`make run`, port 8081) dengan `CORS_ALLOWED_ORIGINS` yang memuat `http://localhost:5181` (`.env.example`-nya sudah memuatnya).
- Satu akun Platform Admin dan satu akun user uji yang menjadi anggota **dua organisasi tenant** (A dan B).

## Checklist setup NICE

Jalankan dari direktori `example/service-client-ui`. Token diambil dengan `nice/scripts/dev-token.sh` (bila akun memakai verifikasi dua langkah, set `TOTP_SECRET`, lihat kepala skrip). Password dibaca dari terminal dan tidak pernah ditulis ke berkas.

```bash
export NICE=${NICE:-http://localhost:8080}   # sama dengan VITE_NICE_ISSUER; dev-token.sh juga membacanya
read -rp 'Email Platform Admin: ' ADMIN_EMAIL; read -rsp 'Password: ' ADMIN_PASSWORD; echo
ADMIN_TOKEN=$(../../nice/scripts/dev-token.sh "$ADMIN_EMAIL" "$ADMIN_PASSWORD"); unset ADMIN_PASSWORD
PLATFORM_ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $ADMIN_TOKEN" | jq -r .organization.id)
```

### 1. Project di organisasi platform

Service client harus milik project **organisasi platform**. Client di organisasi tenant ditolak saat diikat (`400 service_client_platform_only`). Pakai project yang ada, atau buat satu (console: Platform Admin, organisasi platform, **Project**):

```bash
UI_PROJECT_ID=$(curl -sS "$NICE/api/v1/organizations/$PLATFORM_ORG_ID/projects" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Inventaris UI","slug":"inventaris-ui"}' | jq -r .project.id)
```

### 2. OAuth client `public`

Console: project itu, halaman **OAuth client**. Buat client bertipe public dengan redirect URI `http://localhost:5181/callback`, post-logout redirect URI `http://localhost:5181/`, dan grant `authorization_code` serta `refresh_token`.

```bash
CLIENT=$(curl -sS "$NICE/api/v1/projects/$UI_PROJECT_ID/clients" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"service-client-ui","client_type":"CLIENT_TYPE_PUBLIC",
       "redirect_uris":["http://localhost:5181/callback"],
       "post_logout_redirect_uris":["http://localhost:5181/"],
       "grant_types":["authorization_code","refresh_token"]}')
OAUTH_CLIENT_ID=$(jq -r .oauth_client.id <<<"$CLIENT"); jq -r .oauth_client.client_id <<<"$CLIENT"
```

Salin `client_id` ke `VITE_NICE_CLIENT_ID`. Client public tidak punya secret.

### 3. Ikat client ke `inventaris`

Butuh `nice.platform.catalog.manage`.

```bash
curl -sS "$NICE/api/v1/projects/$UI_PROJECT_ID/clients/$OAUTH_CLIENT_ID/service" -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"inventaris"}' \
  | jq '.oauth_client | {client_class, service}'
# {"client_class":"service","service":"inventaris"}
```

### 4. Dependensi `inventaris` ke `nice`

Butuh `nice.platform.catalog.manage`. Deklarasi ini membuka token API NICE terdelegasi untuk client itu, dengan `nice.me.contexts.read` sebagai plafonnya. Daftar diganti utuh; bila `inventaris` sudah punya dependensi lain, sertakan juga.

```bash
curl -sS "$NICE/api/v1/shared-services/inventaris/dependencies" -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"dependencies":[{"service":"nice","permissions":["nice.me.contexts.read"]}]}'
```

### 5. Dua organisasi yang memakai `inventaris`

Untuk organisasi A dan B, jalankan langkah 6-8 README `shared-service-oauth`: entitlement `inventaris`, aktivasi di satu project, dan binding user uji ke `inventaris.operator` di project itu. Tanpa binding, project tidak muncul di `/me/contexts`.

Opsional, untuk melihat organisasi yang ditandai tidak tersedia: organisasi C milik user uji yang di-entitle `inventaris` tetapi user tidak punya binding `inventaris` di project mana pun.

### 6. CORS NICE

Tidak ada yang perlu diubah di `nice/.env`. NICE mengizinkan origin redirect URI setiap OAuth client `public` (`http://localhost:5181`) dalam 60 detik setelah client dibuat. `NICE_CORS_ALLOWED_ORIGINS` tidak perlu memuat origin ini.

## Konfigurasi

```bash
cp .env.example .env    # lalu isi VITE_NICE_CLIENT_ID
```

| Variabel | Wajib | Isi |
| --- | --- | --- |
| `VITE_NICE_ISSUER` | ya | Alamat NICE, persis `NICE_ISSUER`-nya. `http` hanya diterima untuk `localhost`, `127.0.0.1`, dan `[::1]` |
| `VITE_NICE_CLIENT_ID` | ya | `client_id` dari langkah 2 |
| `VITE_REDIRECT_URI` | tidak | Bawaan `<origin>/callback`; harus menunjuk ke `/callback` di origin UI |
| `VITE_POST_LOGOUT_REDIRECT_URI` | tidak | Bawaan `<origin>/` |
| `VITE_INVENTARIS_URL` | ya | Alamat `inventaris`, mis. `http://127.0.0.1:8081`; UI memanggil `GET <alamat>/items` |

Konfigurasi yang hilang atau tidak sah menampilkan halaman galat yang menyebut nama variabelnya, dan login tidak dimulai. Jalankan ulang `npm run dev` setelah mengubah `.env`.

## Menjalankan dan tes

```bash
npm install
npm run dev        # http://localhost:5181 (strictPort)
npm test           # Vitest dengan NICE dan inventaris palsu, tanpa NICE yang berjalan
npm run typecheck
npm run build
```

## Cara kerja

- **Login.** Authorization code + PKCE `S256`, `state`, dan `nonce`, dengan scope dasar `openid profile email offline_access` saja, tanpa `nice:org`. NICE menyelesaikan login di organisasi pertama user (urut waktu bergabung) yang meng-entitle `inventaris`. Bila tidak ada, NICE menjawab `access_denied` ke `/callback`. UI menjelaskan bahwa tidak ada organisasi user yang memakai service ini dan menawarkan **Masuk dengan akun lain** (`prompt=select_account`).
- **Scope dasar di setiap refresh.** NICE mengganti scope grant dengan scope yang diminta pada setiap refresh. Hanya `nice:org`, `nice:aud`, dan `nice:prj` yang boleh di luar scope grant. Karena itu setiap refresh mengirim scope dasar, supaya token client untuk userinfo tetap bisa diambil.
- **Organisasi sesi.** Refresh token terikat ke satu organisasi. Refresh tanpa `nice:org` tetap di organisasi itu, dan refresh dengan `nice:org:<org>` memindahkannya. UI membaca organisasi sesi dari klaim `nice_org_id` token terakhir yang diterbitkan, dan menambah `nice:org` hanya bila organisasi tujuan berbeda. Keputusan itu diambil di dalam antrean refresh, tepat sebelum request dikirim, karena refresh sebelumnya di antrean bisa sudah memindahkan sesi.
- **Konteks.** Setelah login dan setiap muat ulang: token client (`aud` = `client_id`) untuk `/oauth/userinfo`, lalu token terdelegasi lewat refresh `nice:aud:nice` (tanpa `nice:prj`), lalu `GET /api/v1/me/contexts?page_size=50`, mengikuti `page.next_page_token`. Query `service` tidak dikirim, karena NICE menjawabnya `400 invalid_argument` untuk token terdelegasi; service diambil dari klaim `nice_service`. Pemilih hanya berisi project dari jawaban itu. Organisasi dengan `projects` kosong ditampilkan tidak tersedia.
- **Pindah konteks.** Memilih project menjalankan satu refresh: scope dasar, `nice:org:<org>` bila organisasinya berubah, `nice:aud:inventaris`, dan `nice:prj:<project>` (wajib untuk service client). Halaman tidak berpindah. Bila NICE menolak (`access_denied` untuk organisasi yang tidak boleh dipakai, `invalid_scope` untuk project yang tidak lagi mengaktifkan `inventaris`), refresh token tidak diputar. UI tetap di konteks sebelumnya, menampilkan alasannya, dan memuat ulang `/me/contexts`.
- **Token per konteks.** Kunci `"<org>|<service>|<project>"`. Token dipakai ulang selama `now < iat + 10 menit + jitter(0-60 detik)` dan `now < exp - 60 detik`, jadi kembali ke konteks sebelumnya dalam 10 menit tidak mengirim request token. Sesi tetap di organisasi terakhir sampai refresh berikutnya ke organisasi lain.
- **Galat dan rotasi** mengikuti `public-client-pkce`: satu refresh sekaligus dengan refresh token terbaru, `invalid_grant`/`invalid_client` mengakhiri sesi, jaringan/5xx/429 memakai token lama dengan backoff 5-60 detik, dan 401 `token_stale`/`expired` dari service diulang sekali dengan token baru.

### Penyimpanan

| Data | Tempat |
| --- | --- |
| Access token (client, NICE terdelegasi, per konteks `inventaris`) | memori saja |
| Identitas, jawaban `/me/contexts`, konteks aktif | memori saja; diambil ulang setelah muat ulang |
| `nice-service-ui.refresh_token`, `nice-service-ui.id_token` | `sessionStorage` |
| `nice-service-ui.login` (`verifier`, `state`, `nonce`) | `sessionStorage`, hanya selama login |

Setelah muat ulang, UI kembali ke organisasi refresh token tanpa project terpilih. Klaim yang ditampilkan didekode di browser dan tidak diverifikasi oleh UI.

## Batas

- Hanya membaca. Token terdelegasi contoh ini hanya membawa `nice.me.contexts.read`.
- Hanya `inventaris` yang dipanggil, walaupun `services` per project ditampilkan.
- Tab yang diduplikasi berbagi refresh token yang sama, dan pemakaian ulang refresh token yang sudah dirotasi mengakhiri sesi di kedua tab (lihat `public-client-pkce`).
- Hanya untuk dev: tidak ada build produksi, kontainer, atau TLS.

## Checklist uji manual

Buka DevTools (tab Network dengan **Preserve log**, dan Application > Session Storage). A dan B adalah organisasi dari langkah 5, P project `inventaris` di A, R project `inventaris` di B.

| # | Requirement | Langkah | Hasil yang diharapkan |
| --- | --- | --- | --- |
| 1 | Login sebagai service client | Tekan **Masuk**, login sebagai user uji | Request authorize memuat `code_challenge_method=S256` dan `scope=openid profile email offline_access` tanpa `nice:org`. Kembali ke `http://localhost:5181/` tanpa `code` di URL. `nice_org_id` di Identitas adalah organisasi pertama user (urut bergabung) yang meng-entitle `inventaris`; organisasi lebih awal tanpa entitlement dilewati. Kartu organisasi itu bertanda **Organisasi saat ini** |
| 2 | Login tanpa organisasi | Login dengan akun yang tidak punya organisasi ber-entitlement `inventaris` | Layar **Service tidak tersedia** menjelaskan bahwa tidak ada organisasi user yang memakai service ini. **Masuk dengan akun lain** membuka authorize dengan `prompt=select_account` |
| 3 | Konteks dari NICE | Lihat bagian Konteks setelah langkah 1 | Satu `POST /oauth/token` dengan `scope=openid profile email offline_access nice:aud:nice`, lalu `GET /api/v1/me/contexts?page_size=50` tanpa `service`, dijawab 200. Pemilih berisi persis organisasi dan project dari jawaban itu (bandingkan dengan tab Response), dengan `services` per project. Kartu token "Token API NICE terdelegasi" menampilkan `perms` `["nice.me.contexts.read"]` dan `nice_service` `inventaris` |
| 4 | Organisasi tanpa project | Dengan organisasi C dari langkah 5 | C tampil "Tidak tersedia" dengan alasan user tidak memegang akses `inventaris` di project mana pun, tanpa tombol |
| 5 | Pindah project | Pilih P di A | Satu `POST /oauth/token` dengan `scope=... nice:aud:inventaris nice:prj:<P>` (tanpa `nice:org` bila sesi sudah di A), lalu `GET http://127.0.0.1:8081/items`. Tombol P bertanda aktif dan daftar barang A tampil |
| 6 | Pindah organisasi | Dari A/P, pilih R di B | Tepat satu `POST /oauth/token` dengan `scope=openid profile email offline_access nice:org:<B> nice:aud:inventaris nice:prj:<R>`, dijawab 200 dengan `refresh_token` baru. Tidak ada request dokumen ke NICE (browser tidak berpindah halaman). Daftar barang B tampil |
| 7 | Kembali ke konteks sebelumnya | Dalam 10 menit, pilih P di A lagi | Tidak ada `POST /oauth/token`; `GET /items` memakai token A/P yang sama dengan langkah 5 |
| 8 | Akses dicabut di antara | Setelah pemilih tampil, nonaktifkan `inventaris` di R (console: project R, **Product & Activation**), lalu pilih R | `POST /oauth/token` dijawab 400 `invalid_scope`. UI tetap di konteks sebelumnya, menampilkan alasannya, dan memanggil `GET /api/v1/me/contexts` lagi; R tidak lagi ditawarkan. Session Storage masih memegang refresh token yang sama (tidak diputar). Aktifkan lagi R sesudahnya |
| 9 | Organisasi dicabut | Cabut entitlement `inventaris` organisasi B, lalu pilih R dari pemilih lama | `POST /oauth/token` dengan `nice:org:<B>` dijawab 400 `access_denied`; hasilnya sama dengan langkah 8, dan B hilang dari pemilih setelah dimuat ulang. Berikan lagi entitlement-nya sesudahnya |
| 10 | Muat ulang | Setelah langkah 6, tekan F5 | Tetap login tanpa redirect. Satu refresh dengan scope dasar saja dijawab token dengan `nice_org_id` B (organisasi refresh token terakhir), lalu `GET /oauth/userinfo` dan `/me/contexts`. Session Storage hanya berisi `nice-service-ui.refresh_token` dan `nice-service-ui.id_token` |
| 11 | Sesi berakhir | Di Session Storage, ubah `nice-service-ui.refresh_token` menjadi `rusak`, lalu pilih konteks yang belum punya token | `POST /oauth/token` dijawab `invalid_grant`; layar **Sesi berakhir**; tidak ada kunci `nice-service-ui.*` tersisa |
| 12 | Logout | Tekan **Keluar** | `POST /oauth/revoke`, lalu navigasi ke `/oauth/end_session` dengan `post_logout_redirect_uri=http://localhost:5181/`; kembali ke halaman **Masuk** |
