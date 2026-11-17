## MODIFIED Requirements

### Requirement: Konfigurasi lewat env
Service SHALL membaca konfigurasi dari variabel lingkungan atau berkas `.env` (issuer NICE, alamat listen, berkas JWKS, origin CORS, API key `nice.service` yang opsional, `client_id` dan `client_secret` OAuth client confidential, origin publik UI (`PUBLIC_URL`) yang menurunkan redirect URI callback, umur sesi, dan direktori hasil build UI; slug `inventaris` tetap karena nama izin bergantung padanya; `DATABASE_URL` PostgreSQL yang opsional, bila kosong data barang disimpan di memori), dan SHALL menyediakan `.env.example` berisi semua variabel dengan placeholder. Bila variabel wajib tidak ada, service SHALL berhenti saat start dengan pesan yang menyebut variabel itu, tanpa menampilkan nilai rahasia.

#### Scenario: Issuer tidak diisi
- **WHEN** service dijalankan tanpa issuer NICE
- **THEN** proses berhenti dengan pesan yang menyebut nama variabel issuer

#### Scenario: Secret client tidak diisi
- **WHEN** service dijalankan tanpa `client_secret`
- **THEN** proses berhenti dengan pesan yang menyebut nama variabelnya dan tidak mencetak nilai rahasia apa pun

### Requirement: Panduan setup manual dan uji smoke
Contoh SHALL memuat README berbahasa Indonesia dan `README.en.md` yang berisi env NICE yang dibutuhkan dan checklist langkah manual di NICE beserta potongan `curl`-nya. Contoh SHALL memuat skrip smoke yang hanya membaca `.env`, mengambil token Service Account (`client_credentials`, `nice:aud:inventaris`), memeriksa jawaban service pada jalur Bearer, dan memeriksa bahwa `GET /auth/session` tanpa cookie dijawab 401 serta `GET /auth/login` mengarahkan ke NICE.

#### Scenario: Smoke berhasil
- **WHEN** checklist sudah dijalankan, service berjalan, dan skrip smoke dijalankan
- **THEN** skrip melaporkan 401 tanpa token, 200 untuk `GET /items` dengan token Service Account, 401 untuk `GET /auth/session` tanpa cookie, dan pengalihan ke NICE dari `GET /auth/login`, lalu keluar dengan kode 0

#### Scenario: Langkah checklist terlewat
- **WHEN** NICE menolak permintaan token dengan `invalid_scope` atau `invalid_client`
- **THEN** skrip keluar dengan kode bukan 0 dan menyebut langkah checklist yang kemungkinan terlewat

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

## ADDED Requirements

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
