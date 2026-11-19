# Design

## Context

Motivasi: lihat `proposal.md`. Fakta yang membentuk rancangan, semuanya sudah diperiksa di kode:

- `nice/pkg/niceclient` sudah menyediakan seluruh mekanisme yang perlu diperlihatkan. `Config{Kind: KindServiceAccount, ClientID: sa_..., ClientSecret: <key>}` membuat `Token` memakai `client_credentials`; `Run` memperbarui token di latar (bawaan 10 menit setelah terbit, token berumur 30 menit); `Do` menambahkan Bearer dan mengulang sekali pada 401 `token_stale` atau `expired`; `invalid_client` menjadi `GrantError{Permanent: true}`, memanggil `OnSessionEnded` sekali, dan `Token` berikutnya langsung gagal sampai `Reset`.
- Untuk Service Account, `Key.Subject` diisi otomatis dari `ClientID` dan `Key.Org` diabaikan. Project selalu milik Service Account, jadi `Key{Audience: "nadi"}` cukup (scope `nice:aud:nadi`, sama dengan `sharedservice-oauth/backend/scripts/smoke.sh`).
- API `nadi`: `GET /devices` (`nadi.devices.read`), `POST /devices` (`nadi.devices.create`), `PUT` (`edit`), `DELETE` (`delete`, sensitif). Galat service berbentuk `{"error","error_description"}`. Role `nadi.editor` memuat baca, buat, dan edit.
- `niceclient` ada sejak commit awal `nice`, jadi versi NICE minimum contoh ini sama dengan jalur Bearer `nadi`: `c7a5086`.
- Aturan repo (`AGENTS.md`): contoh berdiri sendiri tanpa berbagi kode, README dua bahasa, komentar singkat berbahasa Inggris, tes unit tanpa NICE, rahasia tidak pernah tercetak, nomor checklist dirujuk `scripts/smoke.sh`.

## Goals / Non-Goals

**Goals:**

- Kode sekecil mungkin yang tetap memperlihatkan keempat hal yang sering salah: key hanya ke NICE, token dipakai ulang dan diperbarui, `token_stale` diulang sekali, key yang ditolak menghentikan klien.
- Bisa disalin mitra tanpa membuka contoh lain.
- Struktur folder dan pola penamaan sama dengan contoh Go lain dan `nice`, ditegakkan tes arsitektur.
- Setiap pesan galat menunjuk langkah checklist README.

**Non-Goals:**

- Tidak mengimplementasikan ulang protokol token dengan tangan; itu tugas `niceclient`.
- Tidak ada `update` dan `delete`. `DELETE` sensitif dan butuh pemeriksaan online di sisi service; itu tidak menambah pelajaran tentang sisi klien.
- Tidak memanggil service `api_key` (`pengiriman`); jalur proxy dan mirrored key sudah dicakup `shared-service-apikey`.
- Tidak ada rotasi key otomatis, penyimpanan token ke disk, atau banyak Service Account dalam satu proses.
- Tidak masuk `compose.yml`: programnya dijalankan sekali dari terminal, bukan layanan.

## Decisions

### 1. Program baris perintah, bukan server

Tiga perintah: `list`, `create`, `watch`. Pemanggil mesin yang nyata adalah job atau daemon, bukan sesuatu yang didatangi browser, jadi tidak ada port. `list` dan `create` memperlihatkan satu panggilan lengkap; `watch` satu-satunya cara memperlihatkan pembaruan token dan key yang dicabut saat proses masih hidup.

Alternatif: daemon "pelapor telemetri" yang mengubah status perangkat. Ditolak karena butuh izin `edit`, menambah state lokal (id perangkat sendiri), dan mengaburkan inti contoh.

### 2. Memakai `niceclient`, dengan penjelasan `curl` di README

Kode memakai `niceclient.New` + `Run` + `Do`. README memuat padanan `curl` dari satu panggilan, supaya mitra yang tidak memakai Go tahu persis apa yang terjadi di kabel.

Alternatif: menulis `client_credentials` dengan `net/http` saja. Ditolak: aturan repo meminta perbaikan protokol lewat SDK, dan versi tangan akan mengulang cache, backoff, dan penanganan `token_stale` yang sudah teruji di `nice`.

### 3. Susunan kode mengikuti pola baku contoh Go

Strukturnya sama dengan `agent-backend` dan `sharedservice-oauth/backend`, yang meniru `nice/internal/modules` (`nice/AGENTS.md`, "Struktur satu modul" dan "Penamaan"), dipangkas untuk satu resource (`device`) tanpa repository dan facade:

```text
cmd/service-account-client/main.go   tipis: slog, sinyal, memanggil bootstrap.Run, kode keluar
internal/
  domain/                            aturan murni (*_domain.go): slug dan izin nadi, Device dan statusnya,
                                     jenis galat (GrantError, ErrKeyRefused, ErrTemporary, ErrUnreachable, ServiceError)
  app/device/
    usecase/                         satu aksi per berkas: list_devices_usecase.go (ListDevices),
                                     create_device_usecase.go (CreateDevice), watch_devices_usecase.go (WatchDevices),
                                     service_device_usecase.go (type Service), device_contract.go (interface Gateway)
    dto/                             Device, CreateInput, Failure (*_dto.go)
    internal/failure/                pemetaan galat ke Failure: pesan + langkah checklist (*_helper.go)
  adapter/
    cli/                             routes.go (tabel perintah), satu handler per perintah
                                     (list_devices_handler.go, create_device_handler.go, watch_devices_handler.go);
                                     internal/mapper/ untuk mencetak Device dan Failure (*_mapper.go)
    nadi/                            port ke service lewat niceclient (*_gateway.go): nadi_gateway.go,
                                     list_devices_gateway.go, create_device_gateway.go
  platform/config/                   env dan .env
  bootstrap/                         perakitan (bootstrap.go) dan tes yang melintasi lapisan dengan NICE dan service palsu
  arch_test.go                       akhiran nama berkas, satu fungsi per *_usecase.go, dan arah impor
scripts/smoke.sh
Makefile (run, test, smoke), .env.example, .gitignore, README.md, README.en.md
```

Aturan yang ikut dibawa dan ditegakkan `internal/arch_test.go` (salinan kecil milik contoh ini, seperti di dua contoh Go lain, karena contoh tidak berbagi kode):

- Setiap berkas membawa lapisannya di akhiran nama: `_domain`, `_usecase`, `_contract`, `_dto`, `_handler`, `_routes`/`routes.go`, `_gateway`, `_helper`, `_mapper`.
- Satu `*_usecase.go` berisi tepat satu fungsi yang diekspor dan bernama sama dengan berkasnya; `service_device_usecase.go` hanya mendeklarasikan `Service`.
- Ketergantungan mengalir ke dalam: `domain` dan `app` tidak mengimpor `net/http`, `niceclient`, `adapter`, `platform`, atau `bootstrap`. Hanya `adapter/nadi` yang mengenal `niceclient`, dan hanya `bootstrap` yang mengenal semua lapisan.
- Paket use case dan dto diimpor dengan alias eksplisit (`deviceusecase`, `devicedto`).
- `main.go` tidak memuat logika: `bootstrap.Run(ctx, args, os.LookupEnv, ".env", stdout, log)`.

Akhiran `_gateway` (seperti `sharedservice-oauth/backend/internal/adapter/nice`) dipilih, bukan `_client` milik `agent-backend`, karena adapter ini memegang port ke service lain lewat `niceclient`, sama perannya dengan gateway di contoh itu.

Alternatif: susunan datar `internal/{config,nadi,report}`. Ditolak: lebih pendek, tetapi berbeda dari dua contoh Go lain dan dari `nice`, sehingga mitra yang membaca beberapa contoh harus mempelajari dua pola.

### 4. Konfigurasi

| Variabel | Bawaan | Arti |
|---|---|---|
| `NICE_ISSUER` | wajib | alamat publik NICE |
| `NICE_SA_ID` | wajib | id Service Account (`sa_...`) |
| `NICE_SA_API_KEY` | wajib | API key Service Account; hanya dikirim ke `/oauth/token` |
| `NADI_URL` | `http://127.0.0.1:8081` | alamat `sharedservice-oauth/backend` |
| `WATCH_INTERVAL` | `30s` | jeda `watch` |

`.env` dibaca dari direktori kerja dan nilai di lingkungan menang atas berkas, lewat `internal/platform/config` dengan bentuk yang sama dengan `agent-backend` (`config.Load(lookup, dotenv)`). Nama `NICE_SA_*` dipilih, bukan `NICE_CLIENT_ID/SECRET`, supaya pembaca tidak mengira ini OAuth client. Field input perangkat lewat flag `create` (`-name`, `-serial`, `-status`, `-location`), bukan env.

### 5. Pemetaan galat dalam dua langkah

Adapter `nadi` menerjemahkan galat `niceclient` dan jawaban service menjadi galat `domain`, supaya use case tidak mengenal SDK. `app/device/internal/failure` lalu mengubah galat `domain` menjadi `devicedto.Failure` (kode, pesan, langkah checklist), dan `adapter/cli/internal/mapper` mencetaknya serta menentukan kode keluar.

| Asal | Dikenali adapter dari | Galat `domain` | Pesan menunjuk |
|---|---|---|---|
| NICE `invalid_client` | `errors.Is(err, niceclient.ErrSessionEnded)` | `ErrKeyRefused` | langkah Service Account dan API key |
| NICE `invalid_scope` | `GrantError.Code` | `GrantError` | entitlement organisasi dan aktivasi `nadi` di project |
| NICE tidak terjangkau, 5xx, 429 | `niceclient.IsTemporary` | `ErrTemporary` | NICE berjalan dan `NICE_ISSUER` |
| service 400, 401 | status + `error_description` jawaban | `ServiceError` | masukan, atau token (`token_stale`, `wrong_issuer`, `wrong_audience`) |
| service 403 | status + `error_description` jawaban | `ServiceError` | role binding Service Account |
| service tidak terjangkau | galat transport dari `Do` | `ErrUnreachable` | `NADI_URL` dan `make run` di `sharedservice-oauth/backend` |

Pesan hanya memuat kode dan deskripsi galat, tidak pernah body mentah, key, atau token. Body galat service dibaca dengan batas ukuran.

### 6. Perilaku `watch`

`bootstrap` menjalankan `go client.Run(ctx)` untuk pembaruan token di latar; itu urusan adapter, bukan use case. `WatchDevices` adalah loop berjeda `WATCH_INTERVAL` yang memanggil `Gateway` dan mengirim tiap hasil ke fungsi `emit` dari pemanggil. Jedanya lewat kontrak kecil di `device_contract.go` supaya tes tidak menunggu waktu nyata.

- `ErrTemporary`, `ErrUnreachable`, dan `ServiceError` 5xx atau 429 dilaporkan lewat `emit` dan loop berlanjut.
- Kegagalan lain yang tidak pulih tanpa mengubah setup (`invalid_scope`, 403, 401 yang tetap ditolak setelah satu pengulangan) juga menghentikan loop: mengulangnya setiap jeda tidak berguna.
- `ErrKeyRefused` menghentikan loop dan dikembalikan sebagai galat. Setelah ditolak permanen, `niceclient.Token` langsung gagal tanpa menghubungi NICE, jadi use case melihatnya pada panggilan berikutnya; `OnSessionEnded` hanya dipakai untuk satu baris log.
- Konteks yang dibatalkan (`SIGINT`/`SIGTERM`) menghentikan loop tanpa galat, sehingga kode keluar 0.

Log lewat `log/slog` ke stderr; hasil (daftar perangkat) ke stdout, supaya keluaran bisa dipipa.

### 7. Tes

Mengikuti `agent-backend`: tes satuan di samping kode (`config`, `failure`), dan tes yang melintasi lapisan di `internal/bootstrap` dengan perakitan asli.

NICE palsu: `httptest.Server` dengan `/oauth/token` yang menghitung panggilan, memeriksa `grant_type`, `client_id`, `client_secret`, dan `scope`, dan bisa disetel menjawab `invalid_client`, `invalid_scope`, atau 503. Service palsu: `/devices` yang mencatat header tiap request dan bisa disetel menjawab 401 `token_stale` (dengan `WWW-Authenticate`), 403, 400, atau 5xx. Tes memakai `niceclient.Client` asli, bukan mock SDK, sehingga membuktikan perilaku di kabel: berapa kali `/oauth/token` dipanggil dan header apa yang sampai ke service. Jam, `Tick`, `RefreshAfter`, dan `Jitter` `niceclient` disetel dari tes supaya pembaruan token tidak menunggu waktu nyata. Semua tes berjalan dengan `-race`.

Satu tes khusus memeriksa bahwa nilai key palsu tidak muncul di header, query, atau body request mana pun ke service palsu, dan tidak muncul di stdout, stderr, atau teks galat. `internal/arch_test.go` menjaga struktur.

### 8. Smoke dan checklist

`scripts/smoke.sh` memeriksa: NICE menjawab, `nadi` menjawab `/healthz`, lalu menjalankan `go run ./cmd/service-account-client list` dan memeriksa kode keluarnya. Skrip tidak membuat perangkat, supaya aman dijalankan berulang. Checklist README:

1. `nadi` berjalan dan terdaftar (rujuk checklist `sharedservice-oauth/backend`, langkah 1 sampai 7).
2. Service Account tenant di project yang mengaktifkan `nadi`.
3. Binding Service Account ke role yang memuat baca dan buat (`nadi.editor`).
4. API key Service Account, isi `.env`.
5. Smoke.

Checklist uji menambahkan: role hanya baca lalu `create` (403), dan cabut key saat `watch` berjalan.

## Risks / Trade-offs

- [Pencabutan key baru terlihat saat pembaruan token berikutnya, sampai sekitar 10 menit] → README menyatakannya, dan checklist uji menyarankan `WATCH_INTERVAL` pendek serta menjelaskan bahwa token lama tetap diterima `nadi` sampai kedaluwarsa atau feed menandainya. Tes unit menguji jalur ini dengan jam palsu.
- [Contoh bergantung pada setup `sharedservice-oauth`] → README merujuk langkah bernomor contoh itu, bukan menyalinnya; `smoke.sh` memeriksa `/healthz` `nadi` lebih dulu dan menunjuk checklist itu bila gagal.
- [`go.mod` kelima dengan `replace ../../nice`] → sama dengan `agent-backend`; README indeks sudah mensyaratkan `nice/` di sebelah `example/`.
- [Perilaku 403 pada `created_by` untuk Service Account belum pernah diuji manual di repo ini] → tugas verifikasi integrasi memeriksa `created_by` berisi `sa_...` dan mencatat revisi `nice` yang diuji.
- [`agent-backend` dan `public-client-pkce` masih menyebut `inventaris` dan `/items`] → di luar cakupan; contoh baru hanya bicara ke `nadi` dan `/devices`.

## Open Questions

Tidak ada.
