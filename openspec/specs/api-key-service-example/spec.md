# api-key-service-example Specification

## Purpose
Contoh SharedService bermode `api_key` (slug `pengiriman`): service lama yang hanya mengenal API key miliknya sendiri dan dipanggil lewat proxy NICE. Contoh ini juga menyediakan operasi key upstream untuk mode mirrored key, sebagai referensi mitra dan sandbox dev.

## Requirements

### Requirement: Operasi pengiriman
Service SHALL menyediakan operasi pengiriman per tenant: `GET /shipments` (daftar), `POST /shipments` (buat, status awal `created`), dan `POST /shipments/{id}/cancel` (batalkan). Service SHALL NOT memverifikasi JWT atau izin. Izin ditegakkan proxy NICE dari katalog, dan service hanya menerima request yang membawa kredensial sah.

#### Scenario: Buat pengiriman
- **WHEN** request berkredensial sah dengan `X-Nice-Organization: org_A` mengirim `POST /shipments` dengan tujuan yang sah
- **THEN** service menjawab 201 dengan pengiriman berstatus `created` milik org A

#### Scenario: Batal dua kali
- **WHEN** pengiriman yang sudah `cancelled` dibatalkan lagi
- **THEN** service menjawab 409 dengan body `{"error":"already_cancelled"}`

#### Scenario: Pengiriman tidak ada
- **WHEN** request membatalkan id yang tidak ada di tenant-nya
- **THEN** service menjawab 404 dengan body `{"error":"not_found"}`

#### Scenario: Masukan tidak sah
- **WHEN** request membuat pengiriman dengan tujuan kosong atau lebih dari 200 karakter
- **THEN** service menjawab 400 dengan body `{"error":"invalid_argument","error_description":"<field>"}`

### Requirement: Kredensial service wajib
Setiap operasi selain `GET /healthz` dan `GET /openapi.json` SHALL mensyaratkan header `X-Api-Key` yang nilainya sama dengan kredensial service atau dengan mirrored key yang aktif. Request tanpa header atau dengan nilai lain SHALL dijawab 401 `{"error":"unauthorized"}` tanpa memproses request. Perbandingan nilai SHALL tahan terhadap serangan waktu.

#### Scenario: Dipanggil langsung tanpa key
- **WHEN** klien memanggil `GET /shipments` langsung ke service tanpa `X-Api-Key`
- **THEN** service menjawab 401

#### Scenario: Lewat proxy NICE
- **WHEN** proxy NICE meneruskan request dengan `X-Api-Key` berisi kredensial service
- **THEN** service memproses request

### Requirement: Tenant dan pelaku dari header proxy
Service SHALL memakai `X-Nice-Organization` sebagai kunci tenant dan `X-Nice-Principal` sebagai pelaku yang dicatat pada pengiriman yang dibuat. Header ini SHALL dipercaya hanya karena request membawa kredensial sah, yang membuktikan request datang dari proxy. Request berkredensial sah tanpa `X-Nice-Organization` SHALL dijawab 400 `{"error":"missing_organization"}`. Data SHALL disimpan di memori.

#### Scenario: Org lain tidak melihat data
- **WHEN** org A membuat pengiriman, lalu request untuk org B memanggil `GET /shipments`
- **THEN** daftar untuk org B tidak memuat pengiriman org A

#### Scenario: Pelaku dicatat
- **WHEN** request dengan `X-Nice-Principal: sa_X` membuat pengiriman
- **THEN** pengiriman itu mencatat `created_by` `sa_X`

### Requirement: Operasi key upstream untuk mirrored key
Service SHALL menyediakan `POST /keys` (body `{"label"}`), `POST /keys/{key_id}/rotate`, dan `DELETE /keys/{key_id}`, yang hanya menerima kredensial service. Generate dan rotate SHALL menjawab 2xx dengan JSON `{"key_id","key"}`. Rotate SHALL langsung mematikan key lama. Revoke SHALL menjawab 204, atau 404 bila key tidak dikenal.

#### Scenario: Generate untuk Service Account
- **WHEN** NICE memanggil `POST /keys` dengan kredensial service dan `{"label":"sa_X"}`
- **THEN** service menjawab 201 dengan `key_id` dan `key` baru, dan key itu diterima sebagai `X-Api-Key` untuk operasi pengiriman

#### Scenario: Rotate
- **WHEN** NICE memanggil `POST /keys/{key_id}/rotate` untuk key aktif
- **THEN** service menjawab dengan key baru, dan key lama langsung ditolak 401 pada operasi pengiriman

#### Scenario: Revoke key yang sudah tidak ada
- **WHEN** NICE memanggil `DELETE /keys/{key_id}` untuk id yang tidak dikenal
- **THEN** service menjawab 404

#### Scenario: Mirrored key memanggil operasi key
- **WHEN** request ke `POST /keys` membawa mirrored key, bukan kredensial service
- **THEN** service menjawab 403 `{"error":"admin_credential_required"}` dan tidak membuat key

### Requirement: Mirrored key bertahan saat restart
Key yang dibuat lewat operasi key SHALL disimpan persisten di disk, dengan penulisan yang tidak meninggalkan berkas setengah jadi, sehingga key aktif tetap diterima setelah service di-restart.

#### Scenario: Restart
- **WHEN** key dibuat, lalu service di-restart
- **THEN** key itu tetap diterima, dan key yang sudah di-revoke tetap ditolak

### Requirement: Dokumen OpenAPI untuk katalog
Service SHALL menyajikan dokumen OpenAPI 3 di `GET /openapi.json` tanpa autentikasi. Dokumen SHALL memuat ketiga operasi pengiriman dengan `x-nice-permission` `pengiriman.shipments.read`, `pengiriman.shipments.create`, dan `pengiriman.shipments.cancel`. Dokumen SHALL NOT memuat operasi key, `/healthz`, dan `/openapi.json`, supaya operasi key tidak pernah menjadi operasi katalog yang dapat dipanggil lewat proxy.

#### Scenario: Impor ke katalog
- **WHEN** Platform Admin mengimpor isi `/openapi.json` ke katalog service `pengiriman`
- **THEN** pratinjau tidak menolak dokumen dan menghasilkan tepat tiga operasi dengan ketiga izin itu

#### Scenario: Operasi key di luar katalog
- **WHEN** klien memanggil `POST /pengiriman/keys` lewat proxy NICE
- **THEN** proxy menolaknya karena operasi itu tidak ada di katalog, dan service tidak dihubungi

### Requirement: Endpoint health terbuka
Service SHALL menjawab `GET /healthz` dengan status 2xx tanpa kredensial, karena probe health NICE tidak membawa kredensial service.

#### Scenario: Probe tanpa kredensial
- **WHEN** probe NICE memanggil `GET /healthz` tanpa `X-Api-Key`
- **THEN** service menjawab 2xx

### Requirement: Konfigurasi lewat env
Service SHALL membaca konfigurasi dari variabel lingkungan atau `.env` (kredensial service, alamat listen, berkas penyimpanan key), dan SHALL menyediakan `.env.example`. Tanpa kredensial service, service SHALL berhenti saat start dengan pesan yang menyebut variabel itu. Nilai kredensial dan key SHALL NOT muncul di log.

#### Scenario: Kredensial tidak diisi
- **WHEN** service dijalankan tanpa kredensial service
- **THEN** proses berhenti dengan pesan yang menyebut nama variabelnya

### Requirement: Panduan setup manual dan uji smoke
Contoh SHALL memuat README berbahasa Indonesia dan `README.en.md` berisi env NICE yang dibutuhkan, checklist manual di NICE beserta potongan `curl` (registrasi `api_key` dengan kredensial dan tiga operasi key `upstream.key_operations`, impor, aktivasi, entitlement, binding, Service Account dan API key, serta pemeriksaan kunci Service Account yang dibuat NICE), dan skrip smoke yang hanya membaca `.env`. README SHALL menyatakan bahwa operasi key wajib dan bahwa hanya Service Account yang dapat memanggil `pengiriman` lewat proxy.

#### Scenario: Smoke lewat proxy
- **WHEN** checklist sudah dijalankan, service berjalan, dan skrip smoke dijalankan
- **THEN** skrip melaporkan 401 untuk panggilan langsung tanpa key, 401 untuk proxy tanpa token, dan 200 untuk `GET /pengiriman/shipments` lewat proxy dengan token Service Account (`nice:aud:pengiriman`), lalu keluar dengan kode 0

#### Scenario: Langkah checklist terlewat
- **WHEN** NICE menolak permintaan token atau proxy menjawab 404, 403, atau 503
- **THEN** skrip keluar dengan kode bukan 0 dan menyebut langkah checklist yang kemungkinan terlewat

#### Scenario: Kunci Service Account belum aktif
- **WHEN** proxy menjawab 503 `upstream_key_unavailable` karena NICE belum selesai membuat kunci Service Account
- **THEN** skrip mengulang panggilan dengan jeda hingga batas waktu, dan lulus bila kunci menjadi aktif, atau gagal menyebut langkah 12 bila batas habis

#### Scenario: Registrasi tanpa operasi key
- **WHEN** pembaca menjalankan potongan `curl` registrasi dari README
- **THEN** permintaan memuat tiga operasi key dan NICE menerimanya tanpa galat `invalid_key_operations`

### Requirement: Image container
Contoh SHALL menyediakan `Dockerfile` yang menjalankan service sebagai pengguna non-root dengan dependensi runtime saja, listen di semua antarmuka container pada port 8082, menyimpan berkas mirrored key di volume `/data`, dan memeriksa kesehatan lewat `GET /healthz`. README SHALL menyatakan bahwa service di container harus tetap privat (hanya proxy NICE yang menjangkaunya) dan bahwa volume data harus bertahan, karena tanpa berkas key setiap mirrored key yang dipegang NICE dijawab 401.

#### Scenario: Container dijalankan dengan volume
- **WHEN** image dijalankan dengan `SERVICE_CREDENTIAL` terisi dan volume di `/data`
- **THEN** service menjawab `GET /healthz` di port 8082, dan key yang dibuat lewat `POST /keys` tetap diterima setelah container dibuat ulang dengan volume yang sama

#### Scenario: Container tanpa kredensial
- **WHEN** image dijalankan tanpa `SERVICE_CREDENTIAL`
- **THEN** proses berhenti dengan pesan yang menyebut nama variabel itu
