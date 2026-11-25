## MODIFIED Requirements

### Requirement: Daftar service dari konfigurasi
SPA SHALL membaca daftar service dari konfigurasi. Setiap entri berisi slug, endpoint, path `GET` yang dipanggil, dan project yang opsional. Untuk setiap entri, SPA SHALL menyediakan aksi panggil yang menampilkan status HTTP, `error_description` bila ada, dan body jawaban. Daftar SHALL NOT diambil dari API NICE, supaya contoh ini hanya bergantung pada endpoint OIDC NICE: token client tenant tanpa `nice:aud:nice` ditolak API NICE, dan token penawaran `nice:aud:nice` pun ditolak `GET /api/v1/me/contexts`.

#### Scenario: Panggil service
- **WHEN** user menekan "Panggil" pada entri `nadi` dengan path `/devices`
- **THEN** SPA memanggil `<endpoint>/devices` dengan `Authorization: Bearer <token nadi>` dan menampilkan status serta body jawabannya

#### Scenario: Tanpa hak ke service
- **WHEN** NICE menolak permintaan token service dengan `invalid_scope`
- **THEN** SPA menampilkan galat itu pada entri tersebut, dan entri lain tetap dapat dipanggil

## ADDED Requirements

### Requirement: Image container
Contoh SHALL menyediakan `Dockerfile` yang membangun SPA dengan setiap variabel `VITE_*` sebagai build arg, lalu menyajikannya dengan nginx non-root di port 8080. Setiap path tanpa ekstensi, termasuk `/callback`, SHALL dijawab dengan `index.html`; `index.html` SHALL disajikan tanpa cache dan aset ber-hash dengan cache panjang.

#### Scenario: Fallback callback
- **WHEN** browser membuka `/callback?code=...&state=...` pada container
- **THEN** nginx menjawab 200 dengan `index.html`, dan SPA menyelesaikan login

#### Scenario: Konfigurasi saat build
- **WHEN** image dibangun tanpa build arg `VITE_NICE_CLIENT_ID`
- **THEN** SPA di container menampilkan halaman galat konfigurasi yang menyebut variabel itu

### Requirement: Panduan image container
README SHALL menjelaskan cara membangun image: konfigurasi `VITE_*` dibakukan saat build, issuer harus sama persis dengan `NICE_ISSUER`, endpoint service harus `https` yang terjangkau browser, dan OAuth client harus mendaftarkan redirect `https://<host>/callback` serta post-logout `https://<host>/`.

#### Scenario: Mitra membangun image untuk host publik
- **WHEN** mitra mengikuti bagian image di README untuk host `https://<host>`
- **THEN** perintah build menyebut setiap build arg, dan URI yang harus didaftarkan di OAuth client tertulis di README
