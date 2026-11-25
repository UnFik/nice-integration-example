## ADDED Requirements

### Requirement: Image container
Contoh SHALL menyediakan `Dockerfile` yang menjalankan service sebagai pengguna non-root dengan dependensi runtime saja, listen di semua antarmuka container pada port 8082, menyimpan berkas mirrored key di volume `/data`, dan memeriksa kesehatan lewat `GET /healthz`. README SHALL menyatakan bahwa service di container harus tetap privat (hanya proxy NICE yang menjangkaunya) dan bahwa volume data harus bertahan, karena tanpa berkas key setiap mirrored key yang dipegang NICE dijawab 401.

#### Scenario: Container dijalankan dengan volume
- **WHEN** image dijalankan dengan `SERVICE_CREDENTIAL` terisi dan volume di `/data`
- **THEN** service menjawab `GET /healthz` di port 8082, dan key yang dibuat lewat `POST /keys` tetap diterima setelah container dibuat ulang dengan volume yang sama

#### Scenario: Container tanpa kredensial
- **WHEN** image dijalankan tanpa `SERVICE_CREDENTIAL`
- **THEN** proses berhenti dengan pesan yang menyebut nama variabel itu
