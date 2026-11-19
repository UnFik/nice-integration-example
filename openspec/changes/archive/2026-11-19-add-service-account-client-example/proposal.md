# Proposal

## Why

Keempat contoh yang ada tidak memperlihatkan sisi **pemanggil mesin**: integrasi tanpa user yang memegang API key Service Account, menukarnya ke NICE menjadi token, lalu memanggil service `nice_token`. Alur itu sekarang hanya muncul sebagai potongan `curl` di `scripts/smoke.sh` dan README, sehingga mitra yang menulis integrasi server-ke-server tidak punya kode acuan untuk hal yang paling sering salah: key dikirim ke service, token tidak diperbarui, dan `token_stale` atau key yang dicabut tidak ditangani.

SDK-nya sudah ada (`nice/pkg/niceclient` dengan `KindServiceAccount`), tetapi belum dipakai contoh mana pun.

## What Changes

- Contoh kelima `service-account-client/`: program baris perintah Go yang berdiri sendiri dan bertindak sebagai Service Account tenant terhadap service `nadi` (`sharedservice-oauth/backend`).
  - `list`: menampilkan perangkat project milik Service Account.
  - `create`: mendaftarkan satu perangkat.
  - `watch`: berjalan terus dan membaca daftar perangkat berkala, untuk memperlihatkan pembaruan token di latar, pengulangan sekali pada `token_stale`, dan berhenti dengan pesan jelas saat key ditolak permanen.
- API key hanya dikirim ke `/oauth/token` NICE. Service hanya menerima `Authorization: Bearer <token>`.
- Galat dipetakan ke pesan yang menyebut penyebab dan langkah checklist terkait (`invalid_client`, `invalid_scope`, `permission_denied`, NICE atau service tidak terjangkau), tanpa pernah mencetak key atau token.
- Tes unit dengan NICE palsu dan service palsu (tanpa NICE yang berjalan), `scripts/smoke.sh` terhadap NICE yang sudah diatur, `README.md` dan `README.en.md` dengan checklist setup bernomor.
- README indeks (`README.md`, `README.en.md`) dan `AGENTS.md`: contoh kelima, perintah, diagram alur, dan versi NICE minimum.

Tidak ada perubahan perilaku pada contoh lain. Bukan **BREAKING**.

## Capabilities

### New Capabilities

- `service-account-client-example`: contoh pemanggil mesin yang membuktikan API key Service Account ke NICE dan memanggil service `nice_token` dengan token hasilnya, termasuk penanganan pembaruan token, token basi, key yang ditolak, dan izin yang kurang.

### Modified Capabilities

Tidak ada. `nice-token-service-example` tidak berubah: `nadi` sudah menerima token Service Account dan `scripts/smoke.sh`-nya sudah menguji jalur itu.

## Impact

- **Kode baru:** `service-account-client/` (modul Go sendiri, `replace github.com/nice-platform/nice => ../../nice`, hanya bergantung pada `nice/pkg/niceclient` dan pustaka standar). Tidak membuka port.
- **Dokumen:** `README.md`, `README.en.md`, dan `AGENTS.md` di akar; README baru dua bahasa di folder contoh.
- **Ketergantungan saat dijalankan:** NICE dev dan `sharedservice-oauth/backend` yang sudah terdaftar sebagai `nadi`; satu Service Account tenant dengan role `nadi` yang memuat izin baca dan buat, serta API key-nya.
- **Tidak terdampak:** `compose.yml`, `nice`, `nice-console`, dan kontrak NICE. Contoh ini mengikuti `nice/docs/api.md` bagian "Service Account: `client_credentials`" dan `nice/docs/token-verification.md` apa adanya.
