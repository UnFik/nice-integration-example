# Spec Delta

## Purpose

Contoh SPA React yang menjadi public client tenant NICE: login lewat authorization code + PKCE, lalu memanggil service `nice_token` dengan token per service yang diambil lewat refresh token yang berotasi, sebagai referensi mitra dan sandbox dev.

## ADDED Requirements

### Requirement: Login dengan authorization code dan PKCE
SPA SHALL memulai login dengan redirect ke endpoint authorize NICE memakai PKCE `S256`, `state`, dan `nonce` acak, dengan scope `openid profile offline_access`. Di `/callback`, SPA SHALL menolak `state` yang tidak cocok atau sudah dipakai, menukar kode dengan token, dan memeriksa `nonce` ID token. Kegagalan SHALL ditampilkan beserta tombol untuk mencoba lagi.

#### Scenario: Login berhasil
- **WHEN** user menekan "Masuk", login di halaman NICE, lalu dikembalikan ke `/callback` dengan kode dan `state` yang cocok
- **THEN** SPA menukar kode, menyimpan sesi, membuang kode dan `state` dari URL, dan menampilkan halaman utama

#### Scenario: State tidak cocok
- **WHEN** `/callback` dibuka dengan `state` yang tidak dibuat oleh SPA ini atau sudah pernah dipakai
- **THEN** SPA tidak menukar kode dan menampilkan galat dengan tombol masuk lagi

#### Scenario: Permintaan login kedaluwarsa
- **WHEN** NICE mengembalikan `error=temporarily_unavailable` dengan `error_description=authentication_expired`
- **THEN** SPA memulai permintaan otorisasi baru tanpa menampilkan galat

### Requirement: Identitas user ditampilkan
Setelah login, SPA SHALL menampilkan identitas dari `/oauth/userinfo` (paling sedikit `sub`, nama, email, dan `nice_org_id`) dan klaim setiap token yang dimilikinya dalam bentuk terdekode, dengan keterangan bahwa klaim yang ditampilkan tidak diverifikasi oleh SPA.

#### Scenario: Halaman utama
- **WHEN** user sudah login
- **THEN** halaman menampilkan userinfo dan klaim token login, termasuk `aud` yang berisi `client_id` SPA

### Requirement: Daftar service dari konfigurasi
SPA SHALL membaca daftar service dari konfigurasi. Setiap entri berisi slug, endpoint, path `GET` yang dipanggil, dan project yang opsional. Untuk setiap entri, SPA SHALL menyediakan aksi panggil yang menampilkan status HTTP, `error_description` bila ada, dan body jawaban. Daftar SHALL NOT diambil dari API NICE, karena token client non-sistem ditolak API NICE.

#### Scenario: Panggil service
- **WHEN** user menekan "Panggil" pada entri `inventaris` dengan path `/items`
- **THEN** SPA memanggil `<endpoint>/items` dengan `Authorization: Bearer <token inventaris>` dan menampilkan status serta body jawabannya

#### Scenario: Tanpa hak ke service
- **WHEN** NICE menolak permintaan token service dengan `invalid_scope`
- **THEN** SPA menampilkan galat itu pada entri tersebut, dan entri lain tetap dapat dipanggil

### Requirement: Token per service lewat refresh token
SPA SHALL memperoleh token untuk sebuah service dengan grant `refresh_token` yang memuat scope `nice:aud:<slug>` dan, bila diisi, `nice:prj:<project>`, tanpa login ulang. Token SHALL disimpan per pasangan (service, project) dan dipakai lagi selama berumur kurang dari sekitar 10 menit sejak terbit (dengan jitter) dan belum mendekati `exp`.

#### Scenario: Token dipakai lagi
- **WHEN** user memanggil `inventaris` dua kali dalam satu menit
- **THEN** SPA hanya sekali meminta token `inventaris` ke NICE

#### Scenario: Service kedua tanpa login ulang
- **WHEN** user yang sudah memanggil `inventaris` kemudian memanggil service kedua dari konfigurasi
- **THEN** SPA memperoleh token service kedua lewat refresh token tanpa redirect ke NICE

### Requirement: Satu refresh sekaligus
Karena refresh token NICE berotasi, SPA SHALL menjalankan paling banyak satu permintaan `refresh_token` sekaligus per sesi. Refresh token baru dari setiap jawaban SHALL disimpan sebelum token lain dipakai. Permintaan token yang datang bersamaan untuk pasangan yang sama SHALL berbagi satu hasil.

#### Scenario: Dua service dipanggil bersamaan
- **WHEN** user memanggil dua service sekaligus dan keduanya butuh token baru
- **THEN** SPA mengirim dua permintaan refresh berurutan, masing-masing memakai refresh token terbaru, dan tidak ada yang ditolak `invalid_grant`

### Requirement: Pengulangan pada token basi
Bila service menjawab 401 dengan `error_description` `token_stale` atau `expired`, SPA SHALL membuang token pasangan itu, mengambil token baru sekali, dan mengulang request sekali. 401 kedua SHALL ditampilkan apa adanya tanpa pengulangan lagi.

#### Scenario: Akses berubah
- **WHEN** service menjawab 401 `token_stale` setelah binding user diubah
- **THEN** SPA mengambil token baru dan mengulang request sekali, lalu menampilkan hasil pengulangan

### Requirement: Sesi berakhir dan kegagalan sementara
Jawaban `invalid_grant` atau `invalid_client` dari endpoint token SHALL mengakhiri sesi: SPA membuang semua token dan menampilkan "Sesi berakhir" dengan tombol masuk lagi. Kegagalan sementara (jaringan, 5xx, 429) SHALL NOT mengakhiri sesi: token yang masih berlaku tetap dipakai, dan percobaan berikutnya menunggu backoff 5 sampai 60 detik.

#### Scenario: Refresh token dicabut
- **WHEN** NICE menjawab refresh dengan `invalid_grant`
- **THEN** SPA mengakhiri sesi dan menampilkan "Sesi berakhir"

#### Scenario: NICE tidak terjangkau
- **WHEN** refresh gagal karena NICE tidak terjangkau dan token lama belum kedaluwarsa
- **THEN** SPA tetap memanggil service dengan token lama dan sesi tidak berakhir

### Requirement: Penyimpanan token di browser
SPA SHALL menyimpan access token hanya di memori. Refresh token dan ID token SHALL disimpan di `sessionStorage` supaya sesi bertahan saat reload dan hilang saat tab ditutup. Verifier PKCE, `state`, dan `nonce` SHALL disimpan di `sessionStorage` hanya selama login berlangsung. SPA SHALL NOT memakai `localStorage` atau cookie untuk token.

#### Scenario: Reload halaman
- **WHEN** user yang sudah login me-reload halaman
- **THEN** SPA tetap login tanpa redirect ke NICE, dan token service diambil ulang lewat refresh token saat dibutuhkan

#### Scenario: Tab ditutup
- **WHEN** user menutup tab lalu membuka SPA di tab baru
- **THEN** SPA dalam keadaan belum login

### Requirement: Logout
Logout SHALL mencabut refresh token di `/oauth/revoke`, membersihkan semua token dan data sesi, lalu mengarahkan browser ke endpoint end-session NICE dengan `id_token_hint` dan `post_logout_redirect_uri` SPA. Kegagalan pencabutan SHALL NOT menghentikan logout.

#### Scenario: Logout
- **WHEN** user menekan "Keluar"
- **THEN** refresh token dicabut, `sessionStorage` SPA kosong, dan browser kembali ke SPA dalam keadaan belum login setelah end-session NICE

### Requirement: Konfigurasi lewat env
SPA SHALL membaca issuer NICE, `client_id`, redirect URI, post-logout URI, dan daftar service dari variabel lingkungan build, dan SHALL menyediakan `.env.example`. Konfigurasi yang hilang atau tidak sah SHALL ditampilkan sebagai halaman galat yang menyebut variabelnya, tanpa memulai login.

#### Scenario: Client id kosong
- **WHEN** SPA dijalankan tanpa `client_id`
- **THEN** halaman menampilkan galat konfigurasi yang menyebut nama variabel itu

### Requirement: Panduan setup manual dan uji
Contoh SHALL memuat README berbahasa Indonesia dan `README.en.md` yang berisi env NICE yang dibutuhkan (origin SPA di `NICE_CORS_ALLOWED_ORIGINS`), checklist manual pembuatan OAuth client tenant `public` dengan redirect dan post-logout URI SPA, syarat CORS di sisi service yang dipanggil, serta checklist uji manual di browser untuk setiap requirement di atas.

#### Scenario: Mitra mengikuti README
- **WHEN** mitra menjalankan checklist setup lalu checklist uji manual
- **THEN** setiap langkah uji menyebut hasil yang diharapkan sehingga keberhasilannya dapat diperiksa tanpa membaca kode
