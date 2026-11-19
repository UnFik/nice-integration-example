# Tasks

Struktur folder dan penamaan mengikuti `design.md` keputusan 3 (pola `agent-backend` dan `sharedservice-oauth/backend`, turunan `nice/AGENTS.md`). Jalur berkas di bawah relatif terhadap `service-account-client/`.

## 1. Kerangka dan penjaga struktur

- [x] 1.1 Buat modul Go `service-account-client/` (Go 1.27, `replace github.com/nice-platform/nice => ../../nice`, tanpa dependensi selain `nice` dan pustaka standar), `Makefile` (`run`, `test` dengan `-race`, `smoke`), `.gitignore` (`.env`), dan `cmd/service-account-client/main.go` yang hanya menyiapkan `slog`, sinyal, dan memanggil `bootstrap.Run`; verifikasi `go build ./...` lulus
- [x] 1.2 Tulis `internal/arch_test.go` (salinan kecil dari `agent-backend/internal/arch_test.go`, disesuaikan: akhiran `_domain`, `_usecase`, `_contract`, `_dto`, `_handler`, `_routes`/`routes.go` untuk `adapter/cli`, `_gateway` untuk `adapter/nadi`, `_helper`/`_mapper` di bawah `internal/`; satu fungsi terekspor per `*_usecase.go`; `domain` dan `app` tidak mengimpor `net/http`, `niceclient`, `adapter`, `platform`, `bootstrap`); verifikasi `go test ./internal/ -run 'TestFileNames|TestUseCase|TestImports'` lulus dan gagal bila sebuah berkas sengaja diberi akhiran salah
- [x] 1.3 Implementasikan `internal/platform/config` (`Load(lookup, dotenv)`: lingkungan menang atas `.env`; `NICE_ISSUER`, `NICE_SA_ID`, `NICE_SA_API_KEY` wajib; `NADI_URL` bawaan `http://127.0.0.1:8081`; `WATCH_INTERVAL` bawaan `30s`; `LogValue` yang menyamarkan key) dan `.env.example` berisi placeholder saja; verifikasi tes: variabel wajib yang kosong menghasilkan galat yang menyebut namanya, bawaan terisi, dan nilai key tidak muncul di teks galat maupun log konfigurasi

## 2. Domain dan use case

- [x] 2.1 Tulis `internal/domain` (`device_domain.go`: slug `nadi`, izin, `Device`, status; `failure_domain.go`: `GrantError`, `ErrKeyRefused`, `ErrTemporary`, `ErrUnreachable`, `ServiceError`) dan `internal/app/device/dto` (`device_dto.go`, `failure_dto.go`); verifikasi `go build ./...` dan tes arsitektur lulus
- [x] 2.2 Implementasikan `internal/app/device/usecase`: `device_contract.go` (interface `Gateway` dan kontrak jeda), `service_device_usecase.go` (`Service`), `list_devices_usecase.go` (`ListDevices`), `create_device_usecase.go` (`CreateDevice`); verifikasi tes dengan `Gateway` palsu: hasil diteruskan, galat `domain` dikembalikan sebagai `devicedto.Failure`, dan masukan `create` yang tidak sah ditolak sebelum ada panggilan
- [x] 2.3 Implementasikan `watch_devices_usecase.go` (`WatchDevices`): loop berjeda, hasil dan kegagalan sementara dikirim ke `emit`, `ErrKeyRefused` menghentikan loop dengan galat, konteks batal berhenti tanpa galat; verifikasi tes dengan jeda palsu untuk ketiga jalur itu
- [x] 2.4 Implementasikan `internal/app/device/internal/failure` (`*_helper.go`): galat `domain` menjadi `devicedto.Failure` dengan pesan dan langkah checklist untuk `invalid_client`, `invalid_scope`, NICE tidak terjangkau, service 400, 401, 403, dan service tidak terjangkau; verifikasi tes tabel untuk setiap baris pemetaan di `design.md` keputusan 5

## 3. Adapter dan perakitan

- [x] 3.1 Implementasikan `internal/adapter/nadi` (`nadi_gateway.go`, `list_devices_gateway.go`, `create_device_gateway.go`) di atas `niceclient.Do` dengan `Key{Audience: "nadi"}`, menerjemahkan galat SDK dan jawaban service ke galat `domain`, body galat dibaca dengan batas ukuran; verifikasi `go build ./...` dan tes arsitektur lulus (perilaku di kabel diuji di 3.3)
- [x] 3.2 Implementasikan `internal/adapter/cli` (`routes.go` berisi tabel perintah; `list_devices_handler.go`, `create_device_handler.go` dengan flag `-name`, `-serial`, `-status`, `-location`, `watch_devices_handler.go`; `internal/mapper/` untuk mencetak perangkat dan `Failure`) dan `internal/bootstrap/bootstrap.go` (`Run`: config, `niceclient.New` dengan `KindServiceAccount`, `go client.Run(ctx)`, gateway, service, perintah; hasil ke stdout, log ke stderr); verifikasi perintah tak dikenal mencetak cara pakai dan keluar non-nol
- [x] 3.3 Tulis tes lintas lapisan di `internal/bootstrap` dengan `niceclient.Client` asli terhadap NICE palsu dan service palsu: `list` mengirim satu `client_credentials` berscope `nice:aud:nadi` dan `GET /devices` ber-Bearer serta mencetak perangkat; `create` mengirim `POST /devices` dan mencetak id; beberapa panggilan berturut memakai satu token; 401 `token_stale` sekali lalu 200 diulang sekali dengan token baru; 401 `token_stale` dua kali tidak dicoba ketiga kali; 403 keluar non-nol dengan `permission_denied`; `invalid_scope` dan `invalid_client` keluar non-nol dengan pesan langkahnya; verifikasi `go test -race ./internal/bootstrap/...` lulus
- [x] 3.4 Tes `watch` di `internal/bootstrap` dengan jam palsu: token diperbarui setelah waktu pembaruan dan panggilan berikutnya membawa token baru; `invalid_client` saat pembaruan menghentikan `watch` non-nol; NICE 503 dengan token yang belum kedaluwarsa tidak menghentikan panggilan; service tidak terjangkau dicoba lagi pada jeda berikutnya; konteks batal keluar 0; verifikasi tes lulus dengan `-race`
- [x] 3.5 Tes kerahasiaan di `internal/bootstrap`: nilai key palsu tidak muncul di header, query, atau body request mana pun ke service palsu, dan key maupun token tidak muncul di stdout atau stderr untuk jalur sukses dan setiap jalur galat; verifikasi `make test` lulus tanpa NICE dan tanpa `nadi`

## 4. Smoke dan dokumentasi contoh

- [x] 4.1 Tulis `scripts/smoke.sh` dengan gaya skrip smoke lain (`fail`/`ok`, `set -euo pipefail`, baca `.env` saja; periksa NICE, `/healthz` `nadi`, lalu jalankan `list`; pesan gagal menyebut langkah checklist; tidak mencetak key atau token dan tidak menaruhnya di argv); verifikasi `bash -n scripts/smoke.sh` lulus dan, dengan `.env` kosong, skrip gagal dengan pesan yang menyebut variabel dan langkahnya
- [x] 4.2 Tulis `README.md` dan `README.en.md` dengan isi setara dan urutan bagian seperti `agent-backend/README.md`: pembuka dan baris "Diuji terhadap nice revisi", "Struktur folder" (pohon dari `design.md` keputusan 3 beserta alasannya), cara kerja panggilan dan padanan `curl`, "Galat", "Prasyarat", "Konfigurasi", "Menjalankan dan tes", "Checklist setup NICE" bernomor 1 sampai 5 (dengan perintah `curl` pembuatan Service Account, binding `nadi.editor`, dan API key), "Checklist uji" (role hanya baca lalu `create`, cabut key saat `watch` berjalan beserta catatan jeda pembaruan token), "Batas", dan versi NICE minimum `c7a5086`; verifikasi nama variabel sama dengan `.env.example`, pohon folder sama dengan `git ls-files`, nomor langkah sama dengan yang dirujuk `smoke.sh` dan `internal/failure`, dan setiap perintah yang disebut ada di `Makefile`

## 5. Indeks repo

- [x] 5.1 Perbarui `README.md` dan `README.en.md` akar: jumlah contoh, baris tabel `service-account-client/`, diagram alur `Service Account --client_credentials--> NICE /oauth/token` lalu `--Bearer JWT (aud=nadi)--> sharedservice-oauth/backend`, baris versi NICE minimum, dan `replace ../../nice` di prasyarat; verifikasi tautan relatif valid dan kedua bahasa setara
- [x] 5.2 Perbarui `AGENTS.md`: jumlah contoh, baris tabel (tanpa port), blok perintah `cd service-account-client`, kalimat `replace`, dan versi NICE minimum; verifikasi setiap perintah yang disebut ada di `Makefile` contoh

## 6. Verifikasi integrasi terhadap NICE dev

Grup ini butuh NICE dev yang berjalan dan `sharedservice-oauth/backend` yang sudah terdaftar sebagai `nadi`.

- [x] 6.1 Ikuti checklist setup README terhadap NICE dev, lalu jalankan `scripts/smoke.sh`, `create`, dan `list`; verifikasi smoke lulus, perangkat yang dibuat muncul di `list` dengan `created_by` berisi id Service Account, dan catat revisi `nice` di baris "Diuji terhadap nice revisi" kedua README
- [x] 6.2 Jalankan checklist uji: `create` dengan Service Account ber-role hanya baca dijawab `permission_denied` dengan pesan yang menunjuk langkah binding, dan key yang dicabut saat `watch` berjalan menghentikan `watch` dengan pesan `invalid_client`; verifikasi keluaran kedua uji tidak memuat key atau token

## Workflow follow-up

- Kerjakan di worktree `.worktrees/example-add-service-account-client-example` di akar workspace, branch `feat/add-service-account-client-example` dari `main`, dan pindahkan `openspec/changes/add-service-account-client-example/` ke worktree itu.
- Setelah semua tugas tercentang dan tes terkait hijau, arsipkan change ini dan commit di branch worktree.
- Merge `feat/add-service-account-client-example` ke `main`, lalu hapus worktree dan branch-nya (`git branch -d`). Push hanya bila diminta.
