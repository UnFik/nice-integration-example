# service-bff-example Specification

## Purpose
Contoh SharedService fullstack (`inventaris`, backend Go dan UI React) yang backend-nya menangani seluruh OAuth ke NICE sebagai service client confidential, sehingga UI tidak memegang token dan tidak berbicara ke NICE. Menjadi referensi mitra dan sandbox dev untuk pola backend-for-frontend.

## Requirements

### Requirement: Login lewat backend sebagai service client confidential
Backend SHALL melakukan login lewat authorization code flow dengan PKCE `S256` dan `state` terikat sesi lokal, sebagai OAuth client confidential yang diikat ke `inventaris`. `client_secret` SHALL hanya ada di konfigurasi backend dan SHALL NOT muncul di respons, log, atau UI. Access token dan refresh token SHALL NOT dikirim ke browser.

#### Scenario: Login berhasil
- **WHEN** pengguna menekan Masuk dan menyelesaikan login di NICE
- **THEN** backend menukar `code` dengan `client_secret`, menyimpan refresh token di sisi server, mengatur cookie sesi, dan mengarahkan browser ke UI tanpa token di URL, body, atau penyimpanan browser

#### Scenario: Callback tidak cocok
- **WHEN** callback datang dengan `state` yang tidak dikenal, sudah dipakai, atau berasal dari sesi lain
- **THEN** backend menolak callback, tidak membuat sesi, dan tidak menyimpan token

#### Scenario: Tidak ada organisasi yang memakai service
- **WHEN** NICE menjawab callback dengan `access_denied`
- **THEN** UI menjelaskan bahwa tidak ada organisasi pengguna yang memakai service dan menawarkan masuk dengan akun lain

### Requirement: Sesi sisi server
Backend SHALL mengidentifikasi sesi dengan cookie berisi ID acak, `HttpOnly`, `SameSite=Lax`, dan `Secure` bila diakses lewat HTTPS. Sesi SHALL menyimpan konteks aktif `{org, project}`. Bila NICE mengakhiri sesi (`invalid_grant`), backend SHALL menghapus sesi dan menjawab request berikutnya 401 `session_ended`.

#### Scenario: Request tanpa sesi
- **WHEN** browser memanggil `GET /auth/session` tanpa cookie sesi yang sah
- **THEN** backend menjawab 401 dan UI menampilkan tombol Masuk

#### Scenario: Sesi NICE berakhir
- **WHEN** NICE menjawab `invalid_grant` saat backend me-refresh token sebuah sesi
- **THEN** backend menghapus sesi itu dan UI menampilkan bahwa sesi berakhir serta tombol Masuk

#### Scenario: Backend restart
- **WHEN** backend dijalankan ulang dan penyimpanan sesinya memori
- **THEN** browser yang membawa cookie lama dijawab 401 dan pengguna masuk lagi

### Requirement: Perlindungan CSRF
Backend SHALL menolak dengan 403 setiap request tulis (`POST`, `PUT`, `DELETE`) ber-cookie sesi yang tidak membawa header kustom yang ditetapkan sesi atau yang `Origin`-nya bukan origin aplikasi. Request ber-`Authorization: Bearer` tanpa cookie SHALL tidak terkena aturan ini.

#### Scenario: Form dari situs lain
- **WHEN** halaman di origin lain mengirim `POST /items` ber-cookie sesi tanpa header kustom
- **THEN** backend menjawab 403 dan tidak mengubah data

#### Scenario: Pemanggil Bearer
- **WHEN** `agent-backend` memanggil `POST /items` dengan token NICE tanpa cookie
- **THEN** backend memprosesnya seperti biasa

### Requirement: Konteks dari NICE lewat backend
Backend SHALL membaca organisasi dan project yang tersedia dari `GET /api/v1/me/contexts` memakai token API NICE terdelegasi yang diperoleh dengan menukar token login sesi (tanpa project) lewat token exchange, mengikuti semua halaman, sehingga pemilih tersedia sejak login. Backend SHALL menyimpan hanya ID organisasi dan project di sesi, dan nama atau label SHALL dibaca dari jawaban itu. UI SHALL membangun pemilih hanya dari jawaban itu, dan organisasi tanpa project yang dapat dipakai SHALL ditampilkan sebagai tidak tersedia.

#### Scenario: Pemilih konteks
- **WHEN** NICE menjawab organisasi A dengan project P dan Q, dan organisasi B dengan project R
- **THEN** `GET /auth/contexts` memuat tepat A/P, A/Q, dan B/R, dan UI menandai konteks aktif

#### Scenario: Organisasi tanpa project
- **WHEN** NICE menjawab organisasi C tanpa project
- **THEN** UI menampilkan C sebagai tidak tersedia dengan alasan pengguna tidak punya akses `inventaris` di project mana pun

#### Scenario: Penemuan gagal
- **WHEN** NICE menolak token exchange atau `/me/contexts`
- **THEN** backend menjawab galat yang menyebut penyebabnya dan UI tetap memakai konteks aktif

### Requirement: Pindah konteks lewat refresh di backend
Backend SHALL memindah konteks sesi dengan satu refresh yang menyebut organisasi, service, dan project tujuan (`nice:org`, `nice:aud:inventaris`, `nice:prj`), tanpa mengalihkan browser. Penolakan NICE SHALL membiarkan konteks dan sesi sebelumnya, tidak mengganti refresh token, dan dijawab dengan alasan yang dapat ditampilkan.

#### Scenario: Pindah organisasi
- **WHEN** pengguna di A/P memilih B/R
- **THEN** backend mengirim satu refresh dengan `nice:org:<B> nice:aud:inventaris nice:prj:<R>`, konteks sesi menjadi B/R, dan UI menampilkan barang B/R tanpa berpindah halaman

#### Scenario: Akses dicabut di antara
- **WHEN** pengguna memilih konteks yang aksesnya dicabut setelah daftar dibaca, dan NICE menjawab `invalid_scope` atau `access_denied`
- **THEN** konteks aktif tetap, UI menampilkan alasannya dan memuat ulang pilihan

#### Scenario: Dua tab satu sesi
- **WHEN** dua request serentak dari satu sesi membutuhkan token yang belum ada
- **THEN** refresh token dipakai satu per satu dan tidak ada sesi yang berakhir karena pemakaian ganda

### Requirement: Validasi token pada setiap request ber-cookie
Untuk request ber-cookie sesi ke operasi barang, backend SHALL mengisi token konteks aktif sebagai `Authorization: Bearer` dan melewatkannya pada verifikasi, izin, dan pemisahan tenant yang sama dengan request Bearer langsung. Tenant SHALL berasal dari `nice_org_id` token, bukan dari cookie, header, parameter, atau body.

#### Scenario: Izin dari token
- **WHEN** pengguna yang token konteksnya hanya memuat `inventaris.items.read` membuat barang
- **THEN** backend menjawab 403 `permission_denied` dan tidak membuat barang

#### Scenario: Konteks menentukan tenant
- **WHEN** pengguna pindah dari A/P ke B/R lalu memanggil `GET /items`
- **THEN** daftar memuat barang tenant B dan tidak memuat barang tenant A

### Requirement: UI hanya berbicara ke backend service
UI SHALL mengirim semua request ke origin backend yang sama (`/auth/*` dan operasi barang), dengan cookie dan header CSRF, dan SHALL NOT mengirim request ke NICE, menyimpan token, atau memuat pustaka protokol OAuth. UI SHALL menampilkan tombol aksi sesuai izin yang diumumkan `GET /auth/session`.

#### Scenario: Tidak ada lalu lintas ke NICE
- **WHEN** pengguna masuk, memilih konteks, dan mengubah barang
- **THEN** panel jaringan browser tidak menunjukkan request ke origin NICE kecuali navigasi ke halaman login NICE saat masuk dan keluar

#### Scenario: Penyimpanan browser bersih
- **WHEN** pengguna sudah masuk
- **THEN** `localStorage` dan `sessionStorage` tidak memuat access token, refresh token, atau ID token

### Requirement: Keluar mencabut sesi
`POST /auth/logout` SHALL mencabut refresh token di NICE, menghapus sesi, dan menghapus cookie. Bila NICE tidak terjangkau, backend SHALL mempertahankan sesi dan menjawab galat supaya pengguna tidak tampak keluar saat token masih berlaku.

#### Scenario: Keluar berhasil
- **WHEN** pengguna menekan Keluar
- **THEN** refresh token dicabut di NICE, cookie dihapus, dan request berikutnya dijawab 401

#### Scenario: NICE tidak terjangkau
- **WHEN** NICE tidak menjawab saat Keluar
- **THEN** backend menjawab galat sementara, sesi tetap ada, dan UI menawarkan mencoba lagi

### Requirement: Satu origin
Dalam pengembangan, server dev UI SHALL meneruskan `/auth/*` dan operasi barang ke backend sehingga browser hanya melihat satu origin. Backend SHALL dapat menyajikan hasil build UI dari origin yang sama. Redirect URI client NICE SHALL berupa callback pada origin yang dilihat browser.

#### Scenario: Dev
- **WHEN** UI dibuka di server dev
- **THEN** panggilan ke `/auth/session` dan `/items` melewati origin server dev tanpa preflight CORS

#### Scenario: Build
- **WHEN** backend dijalankan dengan hasil build UI tersedia
- **THEN** `GET /` menyajikan UI dan `GET /auth/session` dijawab backend di origin yang sama

### Requirement: Panduan setup dan uji
Contoh SHALL memuat `README.md` dan `README.en.md` dengan versi NICE minimum, checklist setup manual (OAuth client confidential di project organisasi platform yang diikat ke `inventaris` dengan redirect URI callback backend, dua organisasi yang memakai `inventaris`), dan checklist uji browser dengan hasil yang diharapkan per requirement di atas.

#### Scenario: Mitra mengikuti README
- **WHEN** mitra mengikuti checklist setup dan uji
- **THEN** tiap langkah menyebut hasil yang diharapkan, termasuk request refresh yang terlihat di log NICE saat pindah organisasi dan tidak adanya request ke NICE di panel jaringan browser
