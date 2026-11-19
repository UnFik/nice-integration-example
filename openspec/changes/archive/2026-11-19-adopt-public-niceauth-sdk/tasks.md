# Tasks

Worktree: `.worktrees/example-adopt-public-niceauth-sdk`, branch `feat/adopt-public-niceauth-sdk` dari `main`.

## 1. Prasyarat

- [x] 1.1 Pastikan tag `v0.1.0` ada di remote repo SDK dan change `adopt-public-niceauth-sdk` di `nice` sudah digabung ke `fikri/dev`. Verifikasi: `git ls-remote --tags https://github.com/ABA-Developer/nice-auth-sdk.git` menampilkan `v0.1.0`, dan `nice/pkg` tidak ada. Bila salah satu belum, berhenti dan laporkan.

## 2. `service-account-client`

- [x] 2.1 Ganti impor ke `github.com/ABA-Developer/nice-auth-sdk/credentials`, hapus `replace` dan `require` `nice` dari `go.mod`, tambah SDK `v0.1.0`, jalankan `go mod tidy`, dan ganti awalan terlarang di `internal/arch_test.go`. Verifikasi: `make test` lulus; `go mod tidy -diff` kosong; `go.mod` tidak memuat `nice-platform/nice`.
- [x] 2.2 Perbarui `README.md` dan `README.en.md`: jalur impor di teks dan potongan kode, prasyarat, versi SDK. Verifikasi: kedua berkas setara dan potongan kodenya sama dengan kode.

## 3. `agent-backend`

- [x] 3.1 Ganti impor (`credentials` dan `verify`), `go.mod`, `go mod tidy`, dan awalan di `internal/arch_test.go`. Verifikasi: `make test` lulus; `go mod tidy -diff` kosong.
- [x] 3.2 Perbarui `README.md` dan `README.en.md`. Verifikasi: kedua berkas setara.

## 4. `sharedservice-oauth`

- [x] 4.1 `backend`: ganti impor, `go.mod`, `go mod tidy`. (Catatan apply: `internal/arch_test.go` contoh ini tidak pernah menyebut modul SDK, jadi tidak ada awalan yang diganti.) Verifikasi: `make test` lulus; `make swag-check` lulus; `go mod tidy -diff` kosong.
- [x] 4.2 Hapus konteks `nice` dari `backend/Dockerfile` (dua `COPY --from=nice` dan komentarnya) dan dari `additional_contexts` di `compose.yml`. Verifikasi: `docker compose -p example build nadi` berhasil.
- [x] 4.3 Perbarui README `sharedservice-oauth` dan `sharedservice-oauth/backend` dalam dua bahasa. Verifikasi: tiap pasangan setara.

## 5. Indeks

- [x] 5.1 Perbarui `README.md`, `README.en.md`, dan `AGENTS.md` di akar: tabel contoh, tabel versi dengan kolom SDK, prasyarat susunan direktori, aturan "Kode protokol OAuth" dan "Kontrak NICE" (kontrak verifikasi kini di repo SDK). Sesuaikan rujukan sambil lalu di README `public-client-pkce` dan `shared-service-apikey`. Verifikasi: pencarian `pkg/niceauth`, `nice-platform/nice`, dan `replace ../` di luar `openspec/changes/archive` tidak menemukan apa pun, dan tiap sisa `../nice` adalah petunjuk menjalankan NICE.

## 6. Integrasi

- [x] 6.1 Bangun ulang dan jalankan ulang `nadi` (`docker compose -p example up -d --build nadi`), lalu `sharedservice-oauth/backend/scripts/smoke.sh`. Verifikasi: smoke lulus.
- [x] 6.2 `service-account-client`: `make smoke` terhadap stack lokal dengan `.env` yang ada. Verifikasi: smoke lulus; bila key di `.env` tidak berlaku lagi, catat sebagai belum diuji, jangan menandainya lulus.
- [x] 6.3 Perbarui baris "Diuji terhadap nice revisi" di README contoh yang benar-benar diuji, dengan versi SDK `v0.1.0`; `agent-backend` dicatat hanya diuji unit. Verifikasi: tiap baris sesuai dengan yang dijalankan di 6.1 dan 6.2. Catatan apply: baris itu kini memisahkan versi SDK dari revisi server; server lokal adalah image `nice:dev` dari `c7a3e33`, bukan `df58337` seperti tertulis sebelumnya (`df58337` adalah revisi SDK).

## Workflow follow-up

- Archive change setelah semua tugas tercentang, lalu commit di branch worktree (arsipnya ikut ter-commit; `openspec/` ter-track di `example`).
- Merge `feat/adopt-public-niceauth-sdk` ke `main`, hapus worktree dan branch (`git branch -d`). Push hanya bila diminta.
- `CLAUDE.md` di akar workspace masih menyebut `nice/pkg/nicetoken` dan `replace ../../nice`; berkas itu di luar kedua repo, jadi perbarui hanya bila pemilik workspace memintanya.
