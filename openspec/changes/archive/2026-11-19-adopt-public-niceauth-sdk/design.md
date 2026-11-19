# Design

## Context

Lihat `proposal.md` untuk motivasi. Yang membentuk desain:

- Ketiga `go.mod` menulis `require github.com/nice-platform/nice v0.0.0-...` dan `replace` ke `../../nice` (atau `../../../nice`). Karena itu `go.sum` mereka memuat seluruh dependensi monolit `nice`.
- `internal/arch_test.go` tiap contoh melarang `domain` dan `app` mengimpor `net/http` dan awalan `github.com/nice-platform/nice/`.
- `sharedservice-oauth/backend/Dockerfile` menyalin `go.mod`, `go.sum`, dan `pkg/` dari konteks bernama `nice`, yang diberikan `compose.yml` lewat `additional_contexts`.
- README mencatat "Versi NICE minimum" dengan revisi `nice`; sejak `rename-sdk-to-niceauth` revisi itu sebagian mencerminkan versi SDK, bukan fitur server.
- `scripts/smoke.sh` menyebut `cd ../../nice && make dev` sebagai petunjuk menjalankan NICE.

## Goals / Non-Goals

**Goals:**
- Contoh Go bisa dibangun dan diuji unit tanpa repo `nice`.
- Batas arsitektur contoh tetap ditegakkan terhadap modul SDK yang baru.

**Non-Goals:**
- Mengubah perilaku contoh, checklist, atau smoke.
- Menaikkan direktif `go` di `go.mod` contoh.

## Decisions

### 1. Dipin ke `v0.1.0`, tanpa `replace`

Sama dengan `nice`: `require github.com/ABA-Developer/nice-auth-sdk v0.1.0`. Contoh adalah bahan ajar untuk mitra, jadi `go.mod`-nya harus berbentuk persis seperti milik mitra. `go mod tidy` sesudahnya membuang dependensi monolit dari `go.sum`.

### 2. Tes arsitektur menyebut modul baru

Awalan terlarang di `internal/arch_test.go` menjadi `github.com/ABA-Developer/nice-auth-sdk`. Tanpa ini tes tetap hijau tetapi tidak lagi menjaga apa pun, karena awalan lama tidak akan pernah muncul.

### 3. Image `nadi` dibangun dari konteksnya sendiri

Dua baris `COPY --from=nice` dihapus dari `Dockerfile`, dan `nice: ../nice` dihapus dari `additional_contexts` di `compose.yml`. Konteks `web` tetap. `go mod download` di tahap build mengambil SDK dari proxy modul.

### 4. Tabel versi memisahkan SDK dari NICE

Tabel di README akar mendapat kolom versi SDK (`v0.1.0` untuk ketiga contoh Go). Kolom NICE kembali berisi revisi server yang memang dibutuhkan fiturnya, yang sudah tertulis di teks tiap baris (`add-token-exchange`, `add-client-org-switch`, dan seterusnya). Baris "Diuji terhadap nice revisi" di tiap README ditambah versi SDK yang dipakai saat uji.

### 5. Petunjuk menjalankan NICE dibiarkan

Kalimat di `smoke.sh` dan README yang menyebut `nice/` sebagai tempat menjalankan NICE lokal tetap, karena itu petunjuk untuk developer internal, bukan syarat build. Yang dihapus hanya pernyataan bahwa `nice` harus di samping `example` supaya contoh Go bisa dibangun.

## Risks / Trade-offs

- [Tag `v0.1.0` belum ada saat apply] → Berhenti dan tanyakan; tidak memakai pseudo-version.
- [`go mod tidy` menaikkan dependensi tak langsung lain] → Tinjau diff `go.mod`; hanya blok `nice` dan dependensi bawaannya yang boleh hilang.
- [README menyebut prasyarat lama di banyak tempat dan ada yang terlewat] → Tugas verifikasi mencari `../nice`, `pkg/niceauth`, dan `nice-platform/nice` di luar arsip OpenSpec dan meninjau tiap sisa.

## Migration Plan

1. Setelah change `nice` digabung dan tag `v0.1.0` ada: worktree `.worktrees/example-adopt-public-niceauth-sdk`, branch `feat/adopt-public-niceauth-sdk` dari `main`.
2. Satu commit untuk kode dan build, satu untuk dokumen.
3. Merge ke `main` setelah tes dan smoke hijau.

Rollback: revert merge; berlaku hanya bila `nice` juga di-revert, karena `pkg/niceauth` sudah tidak ada.
