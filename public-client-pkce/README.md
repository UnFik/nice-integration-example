# public-client-pkce

[English](README.en.md)

SPA React yang menjadi **OAuth client tenant bertipe public** di NICE. SPA masuk dengan authorization code + PKCE (`S256`), menampilkan identitas user, lalu memanggil satu atau lebih service `nice_token` (mis. `inventaris` dari `../sharedservice-oauth/backend`) dengan **token terpisah per service** yang diambil lewat refresh token yang berotasi.

Diuji terhadap nice revisi: `c7a5086` (2026-11-13)

### Client tenant atau service client

| | Client tenant (contoh ini) | Service client ([`../sharedservice-oauth`](../sharedservice-oauth/README.md), lewat backend) |
| --- | --- | --- |
| Pemilik | project di organisasi tenant | project di organisasi platform, diikat ke satu service |
| Organisasi | hanya organisasi client | setiap organisasi user yang meng-entitle service itu; pindah lewat refresh `nice:org` |
| Project token service | project client; `nice:prj` boleh kosong | `nice:prj` wajib, project mana pun di organisasi token |
| Discovery konteks | tidak ada; service dari `VITE_SERVICES` | `GET /api/v1/me/contexts` dengan token API NICE terdelegasi |

Pakai contoh ini untuk aplikasi milik satu organisasi. UI service first-party yang melayani banyak organisasi memakai pola backend-for-frontend di `sharedservice-oauth`.

## Isi

| Berkas | Isi |
| --- | --- |
| `src/scope.ts` | Scope dasar (`BASE_SCOPE`) yang dipakai login dan setiap refresh, dan scope per service |
| `src/oidc.ts` | Discovery, permintaan login, callback, refresh, userinfo, revoke, dan URL end-session dengan `oauth4webapi` |
| `src/token-manager.ts` | Token client dan token per `slug|project`: antrean satu refresh, pakai ulang, backoff, sesi berakhir, ulang sekali pada token basi. Tanpa React |
| `src/session.ts` | Perekat: login, muat ulang, identitas lewat userinfo, panggil service, sesi berakhir, logout |
| `src/config.ts` | Validasi `VITE_*` |
| `src/ui/`, `src/App.tsx` | Halaman. `App` memeriksa `location.pathname === "/callback"`, tanpa router |

## Prasyarat

- Node 22.18+ dan npm.
- NICE dev berjalan (`cd nice && make dev`, bawaannya `http://localhost:8080`). Untuk NICE di alamat lain, pakai alamat itu di `NICE` di bawah dan di `VITE_NICE_ISSUER`.
- Service yang dipanggil sudah terdaftar, diimpor, aktif, di-entitle ke organisasi tenant, aktif di project, dan user yang akan login punya binding role di project itu. Untuk `inventaris`, ikuti langkah 1-8 di [`../sharedservice-oauth/backend/README.md`](../sharedservice-oauth/backend/README.md), lalu jalankan servicenya (`make run`, port 8081).

## Checklist setup NICE

Jalankan dari direktori `example/public-client-pkce`. Langkah console disebut dengan nama halamannya; potongan `curl` adalah padanannya. Token admin diambil dengan `nice/scripts/dev-token.sh` (bila akun memakai verifikasi dua langkah, set `TOTP_SECRET`, lihat kepala skrip). Password dibaca dari terminal dan tidak pernah ditulis ke berkas.

```bash
export NICE=${NICE:-http://localhost:8080}   # alamat NICE, sama dengan VITE_NICE_ISSUER; dev-token.sh juga membacanya
read -rp 'Email owner organisasi tenant: ' OWNER_EMAIL; read -rsp 'Password: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD"); unset OWNER_PASSWORD

ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .organization.id)
curl -sS "$NICE/api/v1/organizations/$ORG_ID/projects" -H "Authorization: Bearer $OWNER_TOKEN" | jq '.projects[] | {id, name}'
PROJECT_ID=prj_...   # project tenant tempat inventaris aktif
```

### 1. Env NICE

Pada NICE yang memuat change `add-delegated-nice-api` (`nice` `60939ed` atau sesudahnya), tidak ada yang perlu diubah: NICE mengizinkan origin redirect URI setiap OAuth client `public`, termasuk `http://localhost:5180` dari langkah 2, dalam 60 detik setelah client dibuat. `NICE_CORS_ALLOWED_ORIGINS` tidak wajib lagi.

Pada NICE yang lebih lama, tambahkan origin SPA ke `nice/.env`, lalu jalankan ulang NICE. Daftar dipisah koma, persis tanpa garis miring di akhir:

```text
NICE_CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:5180
```

Tanpa origin yang diizinkan, browser menolak jawaban discovery, `/oauth/token`, `/oauth/userinfo`, dan `/oauth/revoke`. `/oauth/authorize` dan `/oauth/end_session` adalah navigasi penuh, jadi tidak butuh CORS.

### 2. Buat OAuth client tenant `public`

Pakai project di **organisasi tenant**, bukan organisasi platform. Client tenant hanya bisa meminta token service untuk project-nya sendiri; tanpa `nice:prj` NICE memakai project client itu.

Console: project tenant, halaman **OAuth client**. Buat client bertipe public dengan:

- redirect URI `http://localhost:5180/callback`
- post-logout redirect URI `http://localhost:5180/`
- grant `authorization_code` dan `refresh_token` (bawaan)

Admin butuh izin `nice.project.oauthclients.manage` di project itu (owner organisasi memilikinya).

```bash
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/clients" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"public-client-pkce","client_type":"CLIENT_TYPE_PUBLIC",
       "redirect_uris":["http://localhost:5180/callback"],
       "post_logout_redirect_uris":["http://localhost:5180/"],
       "grant_types":["authorization_code","refresh_token"]}' | jq '.oauth_client | {id, client_id}'
```

Salin `client_id` ke `VITE_NICE_CLIENT_ID`; untuk OAuth client tenant nilainya sama dengan `id`, berawalan `cli_`. Client public tidak punya secret.

### 3. Hak user

User yang login di SPA harus anggota organisasi tenant itu dan punya binding role service di project (console: halaman **IAM**; untuk `inventaris` lihat langkah 8 README service). Tanpa binding, NICE tetap menerbitkan token, tetapi service menjawab 403 `permission_denied`.

## Konfigurasi SPA

```bash
cp .env.example .env    # lalu isi VITE_NICE_CLIENT_ID
```

| Variabel | Wajib | Isi |
| --- | --- | --- |
| `VITE_NICE_ISSUER` | ya | Alamat NICE, persis `NICE_ISSUER`-nya. `http` hanya diterima untuk `localhost`, `127.0.0.1`, dan `[::1]`; host lain harus `https` |
| `VITE_NICE_CLIENT_ID` | ya | `client_id` dari langkah 2 |
| `VITE_REDIRECT_URI` | tidak | Bawaan `<origin>/callback`; harus menunjuk ke `/callback` di origin SPA |
| `VITE_POST_LOGOUT_REDIRECT_URI` | tidak | Bawaan `<origin>/` |
| `VITE_SERVICES` | ya | Array JSON `{slug, endpoint, path, project?}`. `path` adalah path `GET` yang dipanggil; `project` (`prj_<uuid>`) opsional |

Konfigurasi yang hilang atau tidak sah menampilkan halaman galat yang menyebut nama variabelnya, dan login tidak dimulai. Variabel `VITE_*` dibaca saat `npm run dev` mulai; jalankan ulang setelah mengubah `.env`.

Daftar service sengaja dibaca dari konfigurasi, bukan dari API NICE: token client non-sistem ditolak API NICE (`/api/v1`).

## Menjalankan dan tes

```bash
npm install
npm run dev        # http://localhost:5180 (strictPort)
npm test           # Vitest, tanpa NICE
npm run typecheck
npm run build
```

## Cara kerja

- **Login.** Scope dasar `openid profile email offline_access`; `offline_access` wajib agar NICE menerbitkan refresh token. `state` dan `nonce` acak, PKCE `S256`. Transaksi login (`verifier`, `state`, `nonce`) disimpan di `sessionStorage` dan dihapus begitu `/callback` dibuka, jadi `state` hanya bisa dipakai sekali. Kode dan `state` dibuang dari address bar (`history.replaceState`) sebelum ditukar. `oauth4webapi` memeriksa `state`, klaim ID token, dan `nonce`. Jawaban `error=temporarily_unavailable` dengan `error_description=authentication_expired` (halaman login dibiarkan lebih dari 30 menit) langsung memulai permintaan otorisasi baru.
- **Scope dasar di setiap refresh.** Pada setiap refresh, NICE mengganti scope grant dengan scope yang diminta (`SetCurrentScopes`). Scope yang diminta harus bagian dari scope grant; hanya `nice:aud:*` dan `nice:prj:*` yang boleh di luarnya, dan selebihnya ditolak `invalid_scope`. Karena itu setiap refresh mengirim scope dasar login ditambah `nice:aud`/`nice:prj`. Bila sebuah refresh hanya mengirim `nice:aud:<slug>`, `openid`, `profile`, dan `email` hilang dari grant, dan token untuk userinfo tidak bisa diambil lagi tanpa login ulang. Login dan token manager memakai satu konstanta, `BASE_SCOPE` di `src/scope.ts`. Di sini contoh ini berbeda dari `nice-auth-sdk/credentials`, yang hanya mengirim `nice:aud`/`nice:prj`. Jangan menyalin bagian itu ke client yang masih butuh userinfo.
- **Token per service.** Untuk setiap pasangan (slug, project), SPA mengirim grant `refresh_token` dengan scope dasar ditambah `nice:aud:<slug>` dan, bila diisi, `nice:prj:<project>`. Token disimpan di memori dengan kunci `"<slug>|<project>"` dan dipakai ulang selama `now < iat + 10 menit + jitter(0-60 detik)` dan `now < exp - 60 detik`. Aturan pakai ulang ini meniru `nice-auth-sdk/credentials`.
- **Rotasi.** NICE memutar refresh token pada setiap pemakaian. SPA menjalankan satu refresh sekaligus per tab (antrean promise), membaca refresh token terbaru tepat sebelum mengirim, dan menyimpan refresh token baru ke `sessionStorage` sebelum memeriksa sisa jawaban. Permintaan bersamaan untuk pasangan yang sama berbagi satu hasil.
- **Galat.** `invalid_grant` atau `invalid_client` dari endpoint token mengakhiri sesi: semua token dibuang dan layar "Sesi berakhir" muncul. Jaringan, 5xx, dan 429 dianggap sementara: token lama yang belum kedaluwarsa tetap dipakai dan percobaan berikutnya menunggu 5, 10, 20, 40, lalu 60 detik. Galat lain (mis. `invalid_scope`) hanya tampil pada entri service itu.
- **Token basi.** Bila service menjawab 401 dengan `error_description` `token_stale` atau `expired` (dari body JSON, atau header `WWW-Authenticate` bila service mengeksposnya), SPA membuang token pasangan itu, mengambil token baru sekali, dan mengulang request sekali. 401 kedua ditampilkan apa adanya.
- **Identitas.** Setelah login dan setiap muat ulang, SPA mengambil token client (kunci `client` di token manager, `aud` = `client_id`) lalu memanggil `/oauth/userinfo`. Setelah login, token client adalah access token hasil penukaran kode. Setelah muat ulang, token itu diambil lewat satu refresh dengan scope dasar saja, di antrean refresh yang sama dengan token service dan dengan aturan pakai ulang yang sama. NICE menyusun userinfo dari scope grant token yang dipakai: `profile` memberi `name`, `preferred_username` (berisi email), dan `picture`; `email` memberi `email` dan `email_verified`. Halaman menampilkan email dari `email`; `preferred_username` hanya terlihat di jawaban userinfo lengkap. `nice_org_id` tidak ada di userinfo, jadi diambil dari klaim token client. Identitas tidak disimpan di penyimpanan browser.
- **Logout.** Refresh token dicabut di `/oauth/revoke` (gagal pun logout jalan terus), semua kunci `nice-example.*` dihapus dari `sessionStorage`, lalu browser diarahkan ke `end_session_endpoint` dengan `id_token_hint`, `client_id`, dan `post_logout_redirect_uri`.

### Penyimpanan

| Data | Tempat |
| --- | --- |
| Access token (client dan per service) | memori saja |
| Identitas (jawaban userinfo dan `nice_org_id`) | memori saja; diambil ulang setelah muat ulang |
| `nice-example.refresh_token`, `nice-example.id_token` | `sessionStorage` |
| `nice-example.login` (`verifier`, `state`, `nonce`) | `sessionStorage`, hanya selama login |

SPA tidak memakai `localStorage` atau cookie. Klaim yang ditampilkan didekode di browser dan tidak diverifikasi oleh SPA; service memverifikasi tokennya sendiri.

## Syarat CORS service yang dipanggil

Browser memanggil service langsung, dengan header `Authorization`, jadi service harus menjawab preflight `OPTIONS` dari origin `http://localhost:5180` dengan `Access-Control-Allow-Origin`, `Access-Control-Allow-Methods` yang memuat `GET`, dan `Access-Control-Allow-Headers` yang memuat `Authorization`. Mengekspos `WWW-Authenticate` lewat `Access-Control-Expose-Headers` dianjurkan tetapi tidak wajib, karena SPA juga membaca `error_description` dari body. Contoh `inventaris` sudah melakukannya (`CORS_ALLOWED_ORIGINS=http://localhost:5180` di `.env`-nya).

## Mengapa SPA ini tidak memanggil service `api_key`

Service `api_key` dipanggil lewat proxy NICE di `<NICE>/<slug>/...`, dan CORS NICE hanya mencakup `/api/v1/*`, `/.well-known/openid-configuration`, dan `/oauth/token|keys|userinfo|revoke`. Browser menolak jawaban proxy dari origin lain, jadi service `api_key` hanya bisa dipanggil dari backend.

## Batas

- **Tab yang diduplikasi mengakhiri kedua tab.** Menduplikasi tab menyalin `sessionStorage`, sehingga dua tab memegang refresh token yang sama. Begitu satu tab memakai refresh token yang sudah dirotasi tab lain, NICE menganggapnya replay dan mencabut seluruh keluarga token (audit `token.refresh_reuse`). Tab itu langsung mendapat `invalid_grant`, dan tab satunya mendapat `invalid_grant` pada refresh berikutnya, jadi **kedua** tab menampilkan "Sesi berakhir". Berbagi satu sesi di banyak tab di luar cakupan contoh ini.
- Tanda tangan ID token tidak diverifikasi di SPA; token diterima langsung dari endpoint token, sesuai OIDC untuk alur kode.
- Hanya untuk dev: tidak ada build produksi, kontainer, atau TLS.

## Checklist uji manual

Buka DevTools (tab Network dan Application > Session Storage). Setiap langkah menyebut hasil yang diharapkan.

| # | Requirement | Langkah | Hasil yang diharapkan |
| --- | --- | --- | --- |
| 1 | Konfigurasi | Kosongkan `VITE_NICE_CLIENT_ID`, jalankan ulang `npm run dev`, buka SPA | Halaman "Konfigurasi belum benar" menyebut `VITE_NICE_CLIENT_ID`; tidak ada request ke NICE. Isi lagi lalu jalankan ulang |
| 2 | Login | Tekan **Masuk**, login di NICE | Kembali ke SPA di `http://localhost:5180/` tanpa `code` dan `state` di URL. Session Storage hanya berisi `nice-example.refresh_token` dan `nice-example.id_token`. Request authorize memuat `code_challenge_method=S256` dan `scope=openid profile email offline_access`. Satu `GET /oauth/userinfo` dengan access token hasil penukaran kode, tanpa refresh |
| 3 | Identitas | Lihat halaman utama | Bagian Identitas menampilkan `sub`, nama, email (dari `email`), dan `nice_org_id` dengan keterangan bahwa nilainya dari klaim access token client. `preferred_username` hanya muncul di jawaban userinfo lengkap. Kartu "Access token client" menampilkan `aud` berisi `client_id`. Ada keterangan "tidak diverifikasi oleh SPA" |
| 4 | State tidak cocok | Buka `http://localhost:5180/callback?code=x&state=palsu` | Layar "Login gagal" dengan tombol **Masuk lagi**; tidak ada `POST /oauth/token` |
| 5 | Login kedaluwarsa | Tekan **Masuk**, biarkan halaman login NICE lebih dari 30 menit, lalu login | SPA langsung mengarah ke authorize baru tanpa layar galat |
| 6 | Panggil service | Tekan **Panggil** pada `inventaris` | Satu `POST /oauth/token` dengan `grant_type=refresh_token` dan `scope=openid profile email offline_access nice:aud:inventaris`, lalu `GET http://127.0.0.1:8081/items` dengan `Authorization: Bearer ...`. Panel menampilkan status 200 dan body. Kartu token `inventaris` muncul dengan `aud=inventaris` |
| 7 | Token dipakai lagi | Tekan **Panggil** lagi dalam satu menit | Tidak ada `POST /oauth/token` baru |
| 8 | Service kedua dan dua sekaligus | Tambahkan entri kedua, mis. `inventaris` dengan `"project":"<PROJECT_ID>"`, jalankan ulang, muat ulang SPA, tekan **Panggil semua** | Setelah satu refresh scope dasar untuk identitas saat muat ulang, dua `POST /oauth/token` berurutan (yang kedua mulai setelah yang pertama selesai), `refresh_token` di request kedua sama dengan yang dijawab request pertama, keduanya 200, tidak ada redirect ke NICE |
| 9 | Tanpa hak | Tambahkan entri dengan slug yang tidak aktif di project, mis. `{"slug":"belumada","endpoint":"http://127.0.0.1:8081","path":"/items"}`, lalu **Panggil** | Entri itu menampilkan `invalid_scope`; entri `inventaris` tetap bisa dipanggil |
| 10 | Token basi | Dengan feed `inventaris` aktif (langkah 9 README service), panggil `inventaris`, ubah binding user di halaman **IAM**, tunggu beberapa detik, panggil lagi | `GET /items` pertama dijawab 401 `token_stale`, lalu satu `POST /oauth/token` dan satu `GET /items` ulang; panel menampilkan hasil pengulangan (mis. 200, atau 403 `permission_denied` bila binding dicabut) |
| 11 | Sesi berakhir | Di Session Storage, ubah `nice-example.refresh_token` menjadi `rusak`, lalu tekan **Panggil** pada entri yang belum punya token (atau muat ulang dulu) | `POST /oauth/token` dijawab `invalid_grant`; layar "Sesi berakhir" dengan **Masuk lagi**; tidak ada kunci `nice-example.*` tersisa |
| 12 | NICE tidak terjangkau | Panggil `inventaris`, tunggu 11 menit, blokir `localhost:8080/oauth/token` di DevTools (Network request blocking) atau hentikan NICE, tekan **Panggil** | `POST /oauth/token` gagal, `GET /items` tetap dikirim dengan token lama dan dijawab 200, sesi tidak berakhir. Menekan lagi dalam 5 detik tidak mengirim `POST /oauth/token`. Setelah blokir dicabut, panggilan sesudah backoff mendapat token baru |
| 13 | Muat ulang | Setelah memanggil `inventaris` (langkah 6), tekan F5 | Tetap di halaman utama tanpa redirect ke NICE. Satu `POST /oauth/token` dengan `grant_type=refresh_token` dan `scope=openid profile email offline_access` (tanpa `nice:aud`) dijawab 200, lalu satu `GET /oauth/userinfo`; bagian Identitas tampil lagi dengan email dan `nice_org_id`. Kartu "Access token client" menampilkan token baru dengan `aud` berisi `client_id`. Session Storage tetap hanya berisi `nice-example.refresh_token` dan `nice-example.id_token`. **Panggil** mengambil token service lewat refresh token |
| 14 | Tab ditutup | Tutup tab, buka `http://localhost:5180` di tab baru | Halaman **Masuk** (belum login) |
| 15 | Logout | Tekan **Keluar** | `POST /oauth/revoke` dengan `token_type_hint=refresh_token`, lalu navigasi ke `/oauth/end_session?id_token_hint=...&post_logout_redirect_uri=http://localhost:5180/`; kembali ke halaman **Masuk**; tidak ada kunci `nice-example.*` |
| 16 | Logout saat revoke gagal | Blokir `localhost:8080/oauth/revoke` di DevTools, login, tekan **Keluar** | Sama seperti langkah 15; revoke gagal tidak menahan logout |
| 17 | Tab diduplikasi | Setelah login, duplikasi tab; di tab baru tekan **Panggil**, kembali ke tab lama dan muat ulang; lalu di tab baru tekan **Panggil** pada entri yang belum punya token (atau muat ulang) | Tab lama langsung menampilkan "Sesi berakhir" karena refresh identitasnya memakai refresh token yang sudah dirotasi; tab baru juga menampilkan "Sesi berakhir" pada refresh berikutnya. Kedua tab berakhir |
