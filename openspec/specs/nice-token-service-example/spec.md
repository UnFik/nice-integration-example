# nice-token-service-example Specification

## Purpose
Contoh SharedService bermode `nice_token` (slug `inventaris`) yang dipanggil klien langsung dengan JWT NICE dan memverifikasi token sendiri sesuai kontrak verifikasi token NICE, sebagai referensi mitra dan sandbox dev.

## Requirements

### Requirement: Operasi barang dan izinnya
Service SHALL menyediakan operasi barang per tenant: `GET /items` dan `GET /items/{id}` dengan izin `inventaris.items.read`, `POST /items` dengan `inventaris.items.create`, `PUT /items/{id}` dengan `inventaris.items.edit`, serta `DELETE /items/{id}` dengan `inventaris.items.delete`. Request yang token-nya tidak memuat izin operasi SHALL dijawab 403 dengan `error_description` `permission_denied`, tanpa mengubah data.

#### Scenario: Izin baca
- **WHEN** klien memanggil `GET /items` dengan token sah yang `perms`-nya memuat `inventaris.items.read`
- **THEN** service menjawab 200 dengan daftar barang milik org token itu

#### Scenario: Izin tidak ada
- **WHEN** klien memanggil `POST /items` dengan token sah yang `perms`-nya hanya memuat `inventaris.items.read`
- **THEN** service menjawab 403 `permission_denied` dan tidak membuat barang

#### Scenario: Barang tidak ada
- **WHEN** klien yang berizin memanggil `GET /items/{id}` untuk id yang tidak ada di tenant-nya
- **THEN** service menjawab 404 dengan body `{"error":"not_found"}`

#### Scenario: Masukan tidak sah
- **WHEN** klien yang berizin membuat barang dengan nama kosong, nama lebih dari 120 karakter, atau jumlah negatif
- **THEN** service menjawab 400 dengan body `{"error":"invalid_argument","error_description":"<field>"}` dan tidak membuat barang

### Requirement: Verifikasi token mengikuti kontrak NICE
Setiap request ke operasi barang SHALL diverifikasi dengan aturan `docs/token-verification.md` NICE: RS256 dari JWKS NICE, `iss` sama persis, `aud` tepat satu nilai `inventaris`, masa berlaku dengan skew paling lama 60 detik. Token yang gagal SHALL dijawab 401 dengan header `WWW-Authenticate: Bearer` dan body `{"error":"invalid_token","error_description":"<alasan>"}` tanpa memuat token.

#### Scenario: Tanpa token
- **WHEN** klien memanggil `GET /items` tanpa header `Authorization`
- **THEN** service menjawab 401 `missing_token`

#### Scenario: Token untuk service lain
- **WHEN** klien memanggil dengan token sah yang `aud`-nya `pengiriman`
- **THEN** service menjawab 401 `wrong_audience`

#### Scenario: Token kedaluwarsa
- **WHEN** klien memanggil dengan token yang `exp`-nya lewat lebih dari 60 detik
- **THEN** service menjawab 401 `expired`

### Requirement: Data terpisah per org
Service SHALL memakai `nice_org_id` dari token terverifikasi sebagai satu-satunya kunci tenant, dan SHALL NOT menentukan tenant dari header, parameter, atau body klien. Org yang memanggil untuk pertama kali SHALL dibuatkan tenant baru secara otomatis. Data SHALL disimpan di memori dan hilang saat service berhenti.

#### Scenario: Org lain tidak melihat data
- **WHEN** user org A membuat barang, lalu user org B memanggil `GET /items`
- **THEN** daftar untuk org B tidak memuat barang org A, dan `GET /items/{id}` barang itu oleh org B dijawab 404

#### Scenario: Tenant pertama kali
- **WHEN** org yang belum pernah memanggil mengirim token sah dengan izin baca
- **THEN** service membuat tenant untuk org itu dan menjawab 200 dengan daftar kosong

### Requirement: Hapus barang adalah operasi sensitif
`DELETE /items/{id}` SHALL diperiksa online ke NICE (`POST /api/v1/access-checks`) pada setiap request, selain pemeriksaan `perms`. Bila NICE menolak, service SHALL menjawab 403 `access_denied`. Bila pemeriksaan tidak dapat dilakukan, termasuk saat API key pemeriksa tidak dikonfigurasi, service SHALL menjawab 503 dan tidak menghapus barang.

#### Scenario: Akses sudah dicabut
- **WHEN** token masih memuat `inventaris.items.delete` tetapi binding principal sudah dicabut di NICE
- **THEN** service menjawab 403 `access_denied` dan barang tetap ada

#### Scenario: Pemeriksa tidak dikonfigurasi
- **WHEN** service berjalan tanpa API key `nice.service` dan klien yang berizin memanggil `DELETE /items/{id}`
- **THEN** service menjawab 503 dan barang tetap ada

### Requirement: Pencabutan cepat lewat feed perubahan akses
Bila API key Service Account `nice.service` dikonfigurasi, service SHALL membaca feed perubahan akses NICE terus-menerus (dengan putar ulang saat start) dan SHALL menolak token yang terbit sebelum perubahan akses principal, project, atau org-nya dengan 401 `token_stale`. Tanpa key, service SHALL tetap melayani operasi tidak sensitif dan mencatat peringatan saat start bahwa pencabutan hanya berlaku saat token kedaluwarsa.

#### Scenario: Binding dicabut
- **WHEN** feed berjalan, binding user dicabut di NICE, lalu user memanggil `GET /items` dengan token yang terbit sebelum pencabutan
- **THEN** service menjawab 401 `token_stale`

#### Scenario: Tanpa key feed
- **WHEN** service dijalankan tanpa API key `nice.service`
- **THEN** service mulai, mencatat peringatan, dan tetap melayani `GET /items` untuk token sah

### Requirement: JWKS bertahan saat NICE mati
Service SHALL menyimpan salinan JWKS terakhir yang valid ke disk dan membacanya saat start, sehingga service yang restart ketika NICE tidak terjangkau tetap memverifikasi token yang masih berlaku.

#### Scenario: Restart saat NICE mati
- **WHEN** service pernah mengambil JWKS, lalu NICE dimatikan dan service di-restart
- **THEN** service tetap menerima token sah yang belum kedaluwarsa

### Requirement: Dokumen OpenAPI untuk katalog
Service SHALL menyajikan dokumen Swagger 2.0 di `GET /openapi.json` tanpa autentikasi. Dokumen SHALL memuat setiap operasi barang dengan `x-nice-permission` yang sama dengan izin yang ditegakkan, `x-nice-sensitive: true` hanya pada `DELETE /items/{id}`, dan `x-nice-roles` tingkat dokumen dengan role `operator` (izin read, create, edit). Dokumen SHALL NOT memuat `/healthz` dan `/openapi.json`.

#### Scenario: Impor ke katalog
- **WHEN** Platform Admin mengimpor isi `/openapi.json` ke katalog service `inventaris`
- **THEN** pratinjau tidak menolak dokumen dan menghasilkan lima operasi, empat izin, serta role `inventaris.viewer`, `inventaris.admin`, dan `inventaris.operator`

#### Scenario: Izin dokumen menyimpang dari kode
- **WHEN** izin yang ditegakkan pada sebuah rute berbeda dari `x-nice-permission` rute itu di dokumen
- **THEN** tes service gagal

### Requirement: Endpoint health terbuka
Service SHALL menjawab `GET /healthz` dengan status 2xx tanpa autentikasi dan tanpa memanggil NICE, agar probe health NICE dapat memakainya.

#### Scenario: Probe NICE
- **WHEN** probe NICE memanggil `GET /healthz` tanpa header `Authorization`
- **THEN** service menjawab 2xx

### Requirement: CORS untuk SPA terdaftar
Service SHALL menjawab preflight CORS dan menambahkan `Access-Control-Allow-Origin` hanya untuk origin yang tercantum di konfigurasi, dengan method operasi barang dan header `Authorization` serta `Content-Type`. Preflight SHALL dijawab tanpa token. Origin lain SHALL NOT menerima header CORS.

#### Scenario: Preflight dari SPA
- **WHEN** browser di `http://localhost:5180` mengirim preflight `OPTIONS /items` dengan `Access-Control-Request-Headers: authorization`, dan origin itu terdaftar
- **THEN** service menjawab 204 dengan header CORS untuk origin itu

#### Scenario: Origin tidak terdaftar
- **WHEN** preflight datang dari origin yang tidak terdaftar
- **THEN** respons tidak memuat `Access-Control-Allow-Origin`

### Requirement: Konfigurasi lewat env
Service SHALL membaca konfigurasi dari variabel lingkungan atau berkas `.env` (issuer NICE, alamat listen, berkas JWKS, origin CORS, dan API key `nice.service` yang opsional; slug `inventaris` tetap karena nama izin bergantung padanya), dan SHALL menyediakan `.env.example` berisi semua variabel dengan nilai dev bawaan. Bila variabel wajib tidak ada, service SHALL berhenti saat start dengan pesan yang menyebut variabel itu.

#### Scenario: Issuer tidak diisi
- **WHEN** service dijalankan tanpa issuer NICE
- **THEN** proses berhenti dengan pesan yang menyebut nama variabel issuer

### Requirement: Panduan setup manual dan uji smoke
Contoh SHALL memuat README berbahasa Indonesia dan `README.en.md` yang berisi env NICE yang dibutuhkan dan checklist langkah manual di NICE beserta potongan `curl`-nya. Contoh SHALL memuat skrip smoke yang hanya membaca `.env`, mengambil token Service Account (`client_credentials`, `nice:aud:inventaris`), dan memeriksa jawaban service.

#### Scenario: Smoke berhasil
- **WHEN** checklist sudah dijalankan, service berjalan, dan skrip smoke dijalankan
- **THEN** skrip melaporkan 401 tanpa token, 200 untuk `GET /items` dengan token Service Account, lalu keluar dengan kode 0

#### Scenario: Langkah checklist terlewat
- **WHEN** NICE menolak permintaan token dengan `invalid_scope` atau `invalid_client`
- **THEN** skrip keluar dengan kode bukan 0 dan menyebut langkah checklist yang kemungkinan terlewat
