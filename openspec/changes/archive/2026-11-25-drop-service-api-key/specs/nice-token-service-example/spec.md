## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: Penyelesaian `perms_ref` dengan token yang diteruskan
Service SHALL menerima token yang membawa `perms_ref` sebagai pengganti `perms` dengan meminta daftar izinnya ke NICE (`POST /api/v1/permission-resolutions`) sekali per referensi, meneruskan token itu sebagai `Authorization: Bearer` dengan body `{}`, tanpa API key dan tanpa konfigurasi. Bila NICE menjawab tidak cocok, service SHALL menjawab 401 `token_stale`. Bila daftar tidak dapat diambil, termasuk saat NICE menolak token dengan 401, service SHALL menjawab 503.

#### Scenario: Referensi diselesaikan sekali
- **WHEN** klien memanggil `GET /items` dua kali dengan token ber-`perms_ref` yang sama
- **THEN** NICE menerima satu penyelesaian dengan body `{}` dan token pertama sebagai Bearer, dan kedua panggilan dijawab 200

#### Scenario: NICE menolak token
- **WHEN** NICE menjawab penyelesaian dengan 401
- **THEN** service menjawab 503
