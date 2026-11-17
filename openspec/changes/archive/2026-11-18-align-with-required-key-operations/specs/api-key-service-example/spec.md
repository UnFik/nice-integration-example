## MODIFIED Requirements

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
