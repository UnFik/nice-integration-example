# Design

## Context

Dengan operasi key lengkap, `PerAccountKeys` aktif di proxy NICE: hanya Service Account yang dilayani, dan kunci per SA dibuat NICE lewat `POST /keys` setelah SA memegang izin service (sapuan sekitar 30 detik). Registrasi `api_key` yang tidak memuat `upstream.key_operations` ditolak 400 `invalid_key_operations`. `pengiriman` di dev sudah berjalan dalam keadaan ini (smoke lulus, kunci SA `active`).

## Decisions

**Operasi key masuk ke registrasi di README.** Potongan `curl` langkah 4 menambahkan `upstream.key_operations` dengan `POST /keys`, `POST /keys/{key_id}/rotate`, `DELETE /keys/{key_id}` (kontrak yang sudah dipenuhi service). Butir Console menyebut bagian Operasi kunci upstream di langkah Informasi wizard. Nomor langkah checklist tidak berubah supaya rujukan di `smoke.sh` dan README lain tetap benar: langkah 12 tetap ada tetapi berisi pemeriksaan (daftar kunci Service Account berstatus `active` lewat `GET /api/v1/service-account-keys?service=pengiriman`, butuh `nice.platform.upstreamkeys.manage`), bukan konfigurasi.

**Smoke menunggu kunci.** Kunci SA dibuat asinkron, jadi 503 `upstream_key_unavailable` tepat setelah langkah 10 adalah keadaan normal sementara. Smoke mencoba ulang panggilan proxy sampai 12 kali dengan jeda 5 detik, lalu gagal dengan pesan yang sama seperti sekarang. Jawaban 4xx lain tidak diulang.

**`agent-backend` kehilangan tool `pengiriman`, bukan mengubah modelnya.** Agen ini bertindak atas nama user lewat token exchange. Service `api_key` dengan kunci per SA hanya menerima Service Account, jadi tidak ada cara agen memanggilnya sebagai user. Alternatif: tetap menyediakan tool yang selalu gagal 403 (menyesatkan sebagai contoh); memanggil dengan Service Account milik agen (bukan lagi "atas nama user", dan menghilangkan jejak `created_by` user). Keduanya ditolak. Kode lain tidak berubah: exchange, cache per `(user, organisasi, project, service)`, dan paralelisme per kunci tetap dan tetap diuji, sekarang lewat dua konteks berbeda pada `inventaris`.

**Struktur kode.** Penghapusan mengikuti pola modul yang ada (handler per tool, rute tool, usecase pemanggilan service, konfigurasi). `go test ./...` tetap memakai `-race`, dan `internal/arch_test.go` dijaga hijau.

**Revisi NICE.** Setelah smoke terhadap NICE dev dijalankan, README `shared-service-apikey` mencatat `40ddb20`; `agent-backend` mencatat revisi yang sama bila tes manualnya dijalankan, atau tetap pada revisi yang diuji terakhir bila tidak.

## Risks / Trade-offs

- Contoh `agent-backend` tidak lagi menunjukkan service `api_key`. Dicatat di README beserta alasannya.
- Mengubah `scripts/smoke.sh` menambah waktu tunggu maksimum sekitar 60 detik saat kunci SA belum siap.
