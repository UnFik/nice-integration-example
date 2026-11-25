# nice-token-service-example Specification

## Purpose
Contoh SharedService bermode `nice_token` (slug `inventaris`) yang dipanggil klien langsung dengan JWT NICE dan memverifikasi token sendiri sesuai kontrak verifikasi token NICE, sebagai referensi mitra dan sandbox dev.

## Requirements

### Requirement: Operasi barang dan izinnya
Service SHALL menyediakan operasi barang per org dan project (`nice_org_id` dan `prj` dari token terverifikasi): `GET /items` dan `GET /items/{id}` dengan izin `inventaris.items.read`, `POST /items` dengan `inventaris.items.create`, `PUT /items/{id}` dengan `inventaris.items.edit`, serta `DELETE /items/{id}` dengan `inventaris.items.delete`. Request yang token-nya tidak memuat izin operasi SHALL dijawab 403 dengan `error_description` `permission_denied`, tanpa mengubah data.

#### Scenario: Izin baca
- **WHEN** klien memanggil `GET /items` dengan token sah yang `perms`-nya memuat `inventaris.items.read`
- **THEN** service menjawab 200 dengan daftar barang milik org dan project token itu

#### Scenario: Izin tidak ada
- **WHEN** klien memanggil `POST /items` dengan token sah yang `perms`-nya hanya memuat `inventaris.items.read`
- **THEN** service menjawab 403 `permission_denied` dan tidak membuat barang

#### Scenario: Barang tidak ada
- **WHEN** klien yang berizin memanggil `GET /items/{id}` untuk id yang tidak ada di org dan project token itu
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
Service SHALL memakai `nice_org_id` dari token terverifikasi sebagai kunci tenant dan klaim `prj` sebagai kunci project, dan SHALL NOT menentukan tenant atau project dari header, parameter, atau body klien. Org yang memanggil untuk pertama kali SHALL dibuatkan tenant baru secara otomatis. Operasi barang dengan token tanpa `prj` SHALL dijawab 403 `project_required` tanpa mengubah data. Bila `DATABASE_URL` kosong, data SHALL disimpan di memori dan hilang saat service berhenti.

#### Scenario: Org lain tidak melihat data
- **WHEN** user org A membuat barang, lalu user org B memanggil `GET /items`
- **THEN** daftar untuk org B tidak memuat barang org A, dan `GET /items/{id}` barang itu oleh org B dijawab 404

#### Scenario: Tenant pertama kali
- **WHEN** org yang belum pernah memanggil mengirim token sah dengan izin baca
- **THEN** service membuat tenant untuk org itu dan menjawab 200 dengan daftar kosong

#### Scenario: Project lain tidak melihat data
- **WHEN** user membuat barang di project P milik org A, lalu memanggil `GET /items` dengan token untuk project Q milik org A
- **THEN** daftar tidak memuat barang project P, dan `GET /items/{id}` barang itu dijawab 404

#### Scenario: Project dari klien diabaikan
- **WHEN** request membawa `project_id` di query yang berbeda dari `prj` token
- **THEN** service memakai `prj` token dan mengabaikan nilai dari klien

#### Scenario: Project di body ditolak
- **WHEN** body `POST /items` atau `PUT /items/{id}` membawa `project_id`
- **THEN** service menjawab 400 dengan body `{"error":"invalid_argument","error_description":"body"}` karena field itu tidak dikenal, tidak membuat atau mengubah barang, dan scope tetap hanya dari token

#### Scenario: Token tanpa project
- **WHEN** klien memanggil `GET /items` dengan token sah yang tidak membawa `prj`
- **THEN** service menjawab 403 `project_required`

### Requirement: Hapus barang adalah operasi sensitif
`DELETE /items/{id}` SHALL diperiksa online ke NICE (`POST /api/v1/access-checks`) pada setiap request, selain pemeriksaan `perms`. Pemeriksaan SHALL meneruskan token request itu sendiri sebagai `Authorization: Bearer` dengan body yang hanya memuat `permission`, dan service SHALL NOT memegang API key NICE untuk itu. Bila NICE menolak, service SHALL menjawab 403 `access_denied`. Bila pemeriksaan tidak dapat dilakukan, termasuk saat NICE tidak terjangkau atau menolak token yang diteruskan dengan 401, service SHALL menjawab 503 dan tidak menghapus barang.

#### Scenario: Akses sudah dicabut
- **WHEN** token masih memuat `inventaris.items.delete` tetapi binding principal sudah dicabut di NICE
- **THEN** service menjawab 403 `access_denied` dan barang tetap ada

#### Scenario: Pemeriksa tidak dikonfigurasi
- **WHEN** service berjalan tanpa API key NICE, klien yang berizin memanggil `DELETE /items/{id}`, dan NICE mengizinkan
- **THEN** NICE menerima `POST /api/v1/access-checks` dengan `Authorization: Bearer` berisi token klien itu dan body `{"permission":"inventaris.items.delete"}`, lalu barang dihapus

#### Scenario: NICE menolak token yang diteruskan
- **WHEN** NICE menjawab pemeriksaan dengan 401
- **THEN** service menjawab 503 dan barang tetap ada

### Requirement: Pencabutan cepat lewat feed perubahan akses
Service SHALL selalu membaca feed perubahan akses NICE terus-menerus (dengan putar ulang saat start), tanpa kredensial dan tanpa konfigurasi, dan SHALL menolak token yang terbit sebelum perubahan akses principal, project, atau org-nya dengan 401 `token_stale`. Perubahan dicocokkan lewat kunci tersamar `key` dan `org_key` yang dihitung dari klaim token.

#### Scenario: Binding dicabut
- **WHEN** feed berjalan, binding user dicabut di NICE, lalu user memanggil `GET /items` dengan token yang terbit sebelum pencabutan
- **THEN** service menjawab 401 `token_stale`

#### Scenario: Tanpa key feed
- **WHEN** service dijalankan tanpa API key NICE apa pun
- **THEN** service membaca `GET /api/v1/access-changes` tanpa header `Authorization` dan tidak mencatat peringatan tentang feed yang mati

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
Service SHALL membaca konfigurasi dari variabel lingkungan atau berkas `.env` (issuer NICE, alamat listen, berkas JWKS, origin CORS, `client_id` dan `client_secret` OAuth client confidential, origin publik UI (`PUBLIC_URL`) yang menurunkan redirect URI callback, umur sesi, dan direktori hasil build UI; slug `inventaris` tetap karena nama izin bergantung padanya; `DATABASE_URL` PostgreSQL yang opsional, bila kosong data barang disimpan di memori), dan SHALL menyediakan `.env.example` berisi semua variabel dengan placeholder. Service SHALL NOT membaca API key NICE untuk feed, pemeriksaan online, atau penyelesaian `perms_ref`, dan `.env.example` SHALL NOT memuat `NICE_SERVICE_API_KEY`. Bila variabel wajib tidak ada, service SHALL berhenti saat start dengan pesan yang menyebut variabel itu, tanpa menampilkan nilai rahasia.

#### Scenario: Issuer tidak diisi
- **WHEN** service dijalankan tanpa issuer NICE
- **THEN** proses berhenti dengan pesan yang menyebut nama variabel issuer

#### Scenario: Secret client tidak diisi
- **WHEN** service dijalankan tanpa `client_secret`
- **THEN** proses berhenti dengan pesan yang menyebut nama variabelnya dan tidak mencetak nilai rahasia apa pun

#### Scenario: Tanpa API key service
- **WHEN** `.env.example` dibaca tes konfigurasi
- **THEN** berkas itu tidak memuat `NICE_SERVICE_API_KEY`

### Requirement: Panduan setup manual dan uji smoke
Contoh SHALL memuat README berbahasa Indonesia dan `README.en.md` yang berisi env NICE yang dibutuhkan dan checklist langkah manual di NICE beserta potongan `curl`-nya. Contoh SHALL memuat skrip smoke yang hanya membaca `.env`, mengambil token Service Account (`client_credentials`, `nice:aud:inventaris`), memeriksa jawaban service pada jalur Bearer, dan memeriksa bahwa `GET /auth/session` tanpa cookie dijawab 401 serta `GET /auth/login` mengarahkan ke NICE.

#### Scenario: Smoke berhasil
- **WHEN** checklist sudah dijalankan, service berjalan, dan skrip smoke dijalankan
- **THEN** skrip melaporkan 401 tanpa token, 200 untuk `GET /items` dengan token Service Account, 401 untuk `GET /auth/session` tanpa cookie, dan pengalihan ke NICE dari `GET /auth/login`, lalu keluar dengan kode 0

#### Scenario: Langkah checklist terlewat
- **WHEN** NICE menolak permintaan token dengan `invalid_scope` atau `invalid_client`
- **THEN** skrip keluar dengan kode bukan 0 dan menyebut langkah checklist yang kemungkinan terlewat

### Requirement: Aktor dicatat pada barang
When the verified token carries `act`, the service SHALL record `act.sub` as `acted_by` on items it creates and SHALL return `acted_by` with the item; without `act`, `acted_by` SHALL be absent. The user (`sub`) SHALL remain the creator.

#### Scenario: Barang dibuat agen
- **WHEN** an item is created with a token whose `sub` is user U and `act.sub` is the client of `agen`
- **THEN** the item has `created_by` U and `acted_by` the client of `agen`

#### Scenario: Barang dibuat user langsung
- **WHEN** an item is created with a token without `act`
- **THEN** the item has no `acted_by`

### Requirement: Data barang di PostgreSQL berkunci org dan project
Bila `DATABASE_URL` diisi, service SHALL menyimpan tenant dan barang di PostgreSQL. Tabel tenant SHALL berkunci `org_id` (dari `nice_org_id`). Tabel barang SHALL memuat `org_id`, `project_id` (dari `prj`), `created_by` (dari `sub`), dan `acted_by` bila token membawa `act`, dengan indeks pada `(org_id, project_id)`. Setiap query repository SHALL memfilter dengan `org_id` dan `project_id` yang dibawa kunci scope dari klaim token, dan repository SHALL NOT menerima kunci scope dari input klien. Membuat tenant untuk org baru SHALL idempoten sehingga dua request pertama yang serentak menghasilkan satu tenant. Tabel SHALL NOT memakai foreign key ke data NICE; ID org, project, dan user disimpan sebagai teks.

#### Scenario: Isolasi org dan project
- **WHEN** barang dibuat untuk org A project P, org A project Q, dan org B project R
- **THEN** setiap konteks hanya melihat barangnya sendiri lewat daftar maupun id

#### Scenario: Pembuat dari token
- **WHEN** user `usr_1` membuat barang dengan token yang `sub`-nya `usr_1`
- **THEN** baris barang memuat `created_by` `usr_1`, `org_id` dan `project_id` dari token, dan tidak ada nilai kunci dari body klien

#### Scenario: Tenant pertama serentak
- **WHEN** dua request pertama dari org yang sama tiba bersamaan
- **THEN** tabel tenant memuat satu baris untuk org itu dan kedua request berhasil

#### Scenario: Tanpa DATABASE_URL
- **WHEN** service dijalankan tanpa `DATABASE_URL`
- **THEN** service memakai penyimpanan memori dengan aturan scope yang sama dan tidak membuka koneksi database

### Requirement: Label org dan project berasal dari NICE
Service SHALL menyimpan hanya ID org dan project dan SHALL NOT menyimpan nama atau label keduanya sebagai sumber kebenaran. Nama dan label yang ditampilkan SHALL dibaca dari API NICE (jawaban `/api/v1/me/contexts`) saat dipakai.

#### Scenario: Nama berubah di NICE
- **WHEN** nama project diubah di NICE lalu pemilih konteks dimuat ulang
- **THEN** UI menampilkan nama baru tanpa perubahan data di service

### Requirement: Penyelesaian `perms_ref` dengan token yang diteruskan
Service SHALL menerima token yang membawa `perms_ref` sebagai pengganti `perms` dengan meminta daftar izinnya ke NICE (`POST /api/v1/permission-resolutions`) sekali per referensi, meneruskan token itu sebagai `Authorization: Bearer` dengan body `{}`, tanpa API key dan tanpa konfigurasi. Bila NICE menjawab tidak cocok, service SHALL menjawab 401 `token_stale`. Bila daftar tidak dapat diambil, termasuk saat NICE menolak token dengan 401, service SHALL menjawab 503.

#### Scenario: Referensi diselesaikan sekali
- **WHEN** klien memanggil `GET /items` dua kali dengan token ber-`perms_ref` yang sama
- **THEN** NICE menerima satu penyelesaian dengan body `{}` dan token pertama sebagai Bearer, dan kedua panggilan dijawab 200

#### Scenario: NICE menolak token
- **WHEN** NICE menjawab penyelesaian dengan 401
- **THEN** service menjawab 503
