# Tasks

## 1. Pembaruan token konteks basi

- [x] 1.1 Tambahkan `DropServiceToken(ctx, subject, c, token)` ke port `NICE` (`session_contract.go`) dan implementasinya di `internal/adapter/nice/nice_gateway.go` lewat `niceclient.Client.Invalidate`; perbarui fake di tes use case sesi; verifikasi `go build ./...`
- [x] 1.2 Tambahkan port `Stale` ke `sessionusecase.Service` dan perbarui `ContextToken`: token basi dibuang lalu diminta sekali lagi, token kedua diserahkan apa adanya, galat NICE dipetakan seperti sekarang; verifikasi tes use case untuk empat kasus: tidak basi (satu permintaan), basi lalu segar (dua permintaan, token baru), basi dua kali (tidak ada permintaan ketiga), NICE menolak (`RefusedError`)
- [x] 1.3 Pasang `Stale` dari `nicetoken.Verifier.Verify` (`Description == nicetoken.ReasonStale`) di `internal/bootstrap/session.go`; verifikasi tes bootstrap jalur ber-cookie: setelah feed palsu menandai pengguna, `GET /items` dan `GET /auth/session` berhasil dengan token baru, sedangkan request Bearer langsung dengan token basi tetap `401 token_stale`

## 2. Dokumen

- [x] 2.1 Tambahkan kalimat pembaruan sekali untuk token konteks basi di bagian "Request barang dari UI" `sharedservice-oauth/README.md` dan padanannya di `README.en.md`; verifikasi isi kedua bahasa setara

## 3. Pemeriksaan akhir

- [x] 3.1 Jalankan `make test` dan `make swag-check` di `sharedservice-oauth/backend`; semua hijau

## Workflow follow-up

- Kerjakan di worktree `.worktrees/example-renew-stale-session-token` dari `main`, archive setelah tugas selesai, commit, merge ke `main`, lalu hapus worktree dan branch.
