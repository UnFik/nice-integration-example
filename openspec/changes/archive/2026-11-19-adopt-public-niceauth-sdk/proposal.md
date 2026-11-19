# Proposal

## Why

Tiga contoh Go memakai SDK lewat `replace ../../nice`, jadi mitra tidak bisa membangunnya tanpa repo privat `nice` di sampingnya. SDK sekarang ada di modul publik `github.com/ABA-Developer/nice-auth-sdk`, dan `nice` menghapus salinannya (`pkg/niceauth`) lewat change berpasangan `adopt-public-niceauth-sdk` di sana. Contoh harus pindah ke modul publik itu, sekaligus menjadi bisa dibangun sendiri.

## What Changes

- **BREAKING** `sharedservice-oauth/backend`, `agent-backend`, dan `service-account-client` mengimpor `github.com/ABA-Developer/nice-auth-sdk/{credentials,verify}` pada `v0.1.0`. `replace github.com/nice-platform/nice => ...` dan `require github.com/nice-platform/nice` dihapus dari ketiga `go.mod`.
- Tes arsitektur ketiga contoh melarang `domain` dan `app` mengimpor modul SDK yang baru, bukan `github.com/nice-platform/nice/`.
- `sharedservice-oauth/backend/Dockerfile` dan `compose.yml` tidak lagi butuh konteks build `nice`.
- README dua bahasa, `AGENTS.md`, dan tabel versi: prasyarat "repo `nice` di sebelah `example`" tidak berlaku lagi untuk membangun contoh; tabel mencatat versi SDK di samping versi NICE.
- Tidak ada perubahan perilaku contoh.

## Capabilities

### New Capabilities

Tidak ada.

### Modified Capabilities
- `agent-backend-example`: dua requirement menyebut jalur paket `pkg/niceauth/...`; diganti dengan paket modul publik. Perilakunya tidak berubah.

## Impact

- Kode: 21 berkas `.go` berganti impor; tiga `go.mod` dan `go.sum`; tiga `internal/arch_test.go`.
- Build: `sharedservice-oauth/backend/Dockerfile`, `compose.yml`.
- Dokumen: `README.md`, `README.en.md`, `AGENTS.md` di akar; README `sharedservice-oauth`, `sharedservice-oauth/backend`, `agent-backend`, `service-account-client` dalam dua bahasa; rujukan sambil lalu di README `public-client-pkce` dan `shared-service-apikey`.
- Bergantung pada: tag `v0.1.0` di repo SDK, dan change berpasangan di `nice` sudah digabung.
- NICE yang berjalan tidak berubah; contoh tetap butuh NICE untuk smoke.
