# Tasks

Prasyarat: `rename-sdk-to-niceauth` sudah di-merge di repo `nice` dan checkout `nice` di sebelah `example/` berada pada revisi itu.

## 1. Persiapan

- [x] 1.1 Periksa `../nice/pkg/niceauth/credentials` dan `../nice/pkg/niceauth/verify` ada dan `../nice/pkg/niceclient` tidak ada, lalu catat hash revisi `nice` (`git -C ../nice rev-parse --short HEAD`) untuk dipakai di README; verifikasi ketiga pemeriksaan itu benar, dan berhenti bila tidak

## 2. `sharedservice-oauth/backend`

- [x] 2.1 Ganti impor dan nama paket `nicetoken` menjadi `niceauth/verify` di `internal/bootstrap`, `internal/adapter/rest` (termasuk `auth` dan `device`), dan tes yang memakainya; verifikasi `go build ./...` lulus untuk paket yang sudah diganti
- [x] 2.2 Ganti impor dan nama paket `niceclient` menjadi `niceauth/credentials` di `internal/adapter/nice`, `internal/bootstrap`, dan tes yang memakainya; verifikasi `go build ./...` dan `go vet ./...` lulus, dan `grep -rn "niceclient\|nicetoken" --include='*.go' .` kosong
- [x] 2.3 Jalankan `make test` dan `make swag-check`; verifikasi keduanya lulus dan `docs/swagger.json` tidak berubah
- [x] 2.4 Perbarui `README.md` dan `README.en.md` di `sharedservice-oauth/` dan `sharedservice-oauth/backend/` (nama paket, kutipan log berawalan `niceauth:`, versi NICE minimum); verifikasi kedua bahasa setara dan `grep -n "niceclient\|nicetoken"` atas keempat berkas kosong

## 3. `agent-backend`

- [x] 3.1 Ganti impor dan nama paket di `internal/adapter/niceexchange`, `internal/adapter/mcp`, `internal/bootstrap`, dan tesnya; verifikasi `go build ./...`, `go vet ./...`, dan `make test` lulus, dan `grep -rn "niceclient\|nicetoken" --include='*.go' .` kosong
- [x] 3.2 Perbarui `README.md` dan `README.en.md` (nama paket dan versi NICE minimum); verifikasi kedua bahasa setara, `docs/readme_test.go` lulus, dan `grep` nama lama atas keduanya kosong

## 4. `service-account-client`

- [x] 4.1 Ganti impor menjadi `niceauth/credentials`; ubah `adapter/nadi` supaya menerima `*http.Client` (hapus antarmuka `Doer`) dan `bootstrap` supaya memberi `client.HTTPClient(credentials.Key{Audience: "nadi"})`; pertahankan urutan penerjemahan galat (`GrantError`, `IsTemporary`, lalu `*url.Error`); verifikasi `make test` lulus tanpa mengubah harapan tes lintas lapisan yang ada (jumlah permintaan token, pengulangan `token_stale`, `invalid_client`, `invalid_scope`, NICE mati, `nadi` mati, dan pemeriksaan kerahasiaan)
- [x] 4.2 Perbarui `README.md` dan `README.en.md`: nama paket, bagian "Cara kerja panggilan" memakai `HTTPClient`, bagian baru "bentuk tersingkat" dengan `NewFromEnv` + `HTTPClient`, kutipan log, dan versi NICE minimum; verifikasi kedua bahasa setara, potongan kode di README ter-compile bila ditempel ke berkas uji sementara, dan `grep` nama lama atas keduanya kosong

## 5. Indeks repo

- [x] 5.1 Perbarui `README.md` dan `README.en.md` akar (kolom stack, prasyarat, kalimat kode protokol, tabel versi NICE minimum ketiga contoh Go), `AGENTS.md` (tabel, aturan "Kode protokol OAuth", versi minimum), dan dua kalimat di `public-client-pkce/README.md` serta `README.en.md`; verifikasi `grep -rn "niceclient\|nicetoken" . --include='*.md' --include='*.go' --exclude-dir=node_modules --exclude-dir=archive --exclude-dir=.git` kosong

## 6. Verifikasi terhadap NICE

- [x] 6.1 Bangun ulang image `nadi` terhadap `nice` revisi baru dan jalankan ulang kontainernya, lalu jalankan `sharedservice-oauth/backend/scripts/smoke.sh` dan `service-account-client/scripts/smoke.sh`; verifikasi keduanya lulus dan perbarui baris "Diuji terhadap nice revisi" di README kedua contoh itu dengan hash dari 1.1
- [x] 6.2 Jalankan `make smoke` `agent-backend` bila setup `agen` dan token user tersedia di stack lokal; verifikasi lulus dan perbarui baris "Diuji terhadap", atau, bila setup tidak tersedia, tulis di baris itu bahwa revisi baru hanya diuji dengan tes unit

## Workflow follow-up

- Kerjakan di worktree `.worktrees/example-adopt-niceauth-sdk` di akar workspace, branch `feat/adopt-niceauth-sdk` dari `main`, pindahkan `openspec/changes/adopt-niceauth-sdk/` ke worktree itu, dan buat symlink sementara `.worktrees/nice -> ../nice` selama pengerjaan.
- Setelah semua tugas tercentang dan tes terkait hijau, arsipkan change ini dan commit di branch worktree.
- Merge `feat/adopt-niceauth-sdk` ke `main`, lalu hapus worktree, branch (`git branch -d`), dan symlink-nya. Push hanya bila diminta.
