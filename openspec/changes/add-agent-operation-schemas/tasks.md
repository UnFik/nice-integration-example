# Tasks

Path tanpa awalan relatif terhadap `agent/backend` di worktree `.worktrees/example-add-agent-operation-schemas` (branch `feat/add-agent-operation-schemas` dari `main`). Perintah berat dijalankan dengan prefix `heavy`. Bila `go` gagal karena versi toolchain, dahulukan `~/.local/share/mise/installs/go/1.27.3/bin` di `PATH`. Tes unit tidak butuh NICE.

## 1. Persiapan

- [x] 1.1 Buat worktree dan branch dari `main`, lalu naikkan SDK: `go get github.com/ABA-Developer/nice-auth-sdk@v0.2.5 && go mod tidy`. Verifikasi `heavy make test` lulus tanpa perubahan kode lain.

## 2. Discovery yang lebih kaya dan cache

- [x] 2.1 Tambah `Description` ke `calldto.Operation` dan `OperationView`, serta `Description`, `Format`, `Enum`, dan `EnumTruncated` (JSON `omitempty`) ke `calldto.RequestField`. Salin dari SDK di `niceexchange/discovery_client.go`, dan teruskan di `ListOperations`. Verifikasi dengan tes adapter (isian kaya tersalin; jawaban NICE lama tanpa isian itu tetap terbaca) dan tes use case (`list_operations` memuat `description` dan `enum`, dan isian kosong tidak muncul). Jalankan `heavy go test ./internal/... -count=1 -race`.
- [x] 2.2 Tambah parameter `user` ke `Discoverer.Operations` dan pakai `Caller.Subject` di use case. Buat cache LRU per `(user, slug)` di `niceexchange.Client` (maksimal 1000 entri) yang memakai `ServiceOperationsIfChanged` dan menghapus entri saat galat. Verifikasi dengan tes adapter terhadap NICE palsu `httptest`:
  - panggilan kedua mengirim `If-None-Match` dan memakai simpanan saat 304;
  - 200 dengan `ETag` baru mengganti simpanan;
  - 404 menghapus entri;
  - entri ke-1001 mengusir yang paling lama;
  - dua user tidak berbagi entri.

  Jalankan perintah tes yang sama.

## 3. Skema operasi

- [ ] 3.1 Tambah `calldto.OperationSchemas` dan `OperationSchema`, method `Discoverer.OperationSchemas` dan `Discoverer.Redact` di adapter (membungkus `OperationSchemas`, `ResponseSchema`, dan `RedactSecrets`), serta use case `GetOperationSchemas` (validasi slug, 1–10 key, `schemas_unavailable` untuk 404, galat lain lewat `failure.Discovery`). Verifikasi dengan tes use case: dua key dengan satu `missing`, 0 dan 11 key ditolak tanpa memanggil NICE, 404 menjadi `schemas_unavailable`, dan `input_schema` diteruskan byte demi byte. Jalankan perintah tes yang sama.
- [ ] 3.2 Tambah tool `get_operation_schema` ke tabel `tools` (`readOnly`, deskripsi sesuai design) beserta handler `internal/adapter/mcp/get_operation_schema_handler.go`, dan daftarkan di `newServer`. Verifikasi dengan tes MCP yang ada untuk `tools/list` (empat tool, skema input `operations` berupa array string) dan tes `ToolSpecs` chat. Jalankan perintah tes yang sama.

## 4. Pembersihan rahasia di `call_operation`

- [ ] 4.1 Di `CallOperation`, baca skema operasi sebelum persetujuan sensitif dan sebelum `Operator.CallOperation`. Galat sementara atau tak terjangkau dikembalikan tanpa memanggil service; 404 berarti lanjut tanpa skema; key di `missing` menjadi `invalid_argument`. Bersihkan body dengan `Redact` sebelum decode 2xx dan sebelum `failure.Service`; bila pembersihan gagal pada jawaban galat, kirim body kosong ke `failure.Service`. Verifikasi dengan tes use case:
  - `acted_by` bertanda `x-secret` hilang dari `CallView.Body`, dan properti lain tetap;
  - NICE 503 saat membaca skema menghasilkan galat sementara dan `Operator` tidak dipanggil;
  - NICE 404 saat membaca skema membuat panggilan tetap berjalan dengan body utuh;
  - key di `missing` menghasilkan `invalid_argument` tanpa exchange;
  - operasi sensitif tetap meminta persetujuan setelah skema terbaca.

  Jalankan perintah tes yang sama.

## 5. Dokumen, smoke, dan uji manual

- [ ] 5.1 Perbarui `agent/README.md` dan `agent/README.en.md`: tabel tool, pembersihan `x-secret`, cache `ETag`, versi NICE/SDK, dan baris "Diuji terhadap nice revisi". Perbarui baris `agent/backend` di `README.md` dan `README.en.md` akar, serta kalimat versi SDK di `AGENTS.md`. Perbarui `scripts/smoke.sh`: daftar tool yang diharapkan memuat `get_operation_schema`, dan satu langkah baru memanggilnya untuk key `GET /devices` lalu memeriksa `input_schema` ada. Verifikasi dengan `bash -n scripts/smoke.sh` dan membaca ulang kedua README berdampingan.
- [ ] 5.2 Gerbang: `test -z "$(gofmt -l .)"`, `heavy go vet ./...`, dan `heavy make test`; semuanya harus hijau.
- [ ] 5.3 Jalankan agen terhadap NICE `dev` di clone DB dev dengan katalog `nadi` yang diimpor ulang (salinan dokumen dengan `acted_by` bertanda `x-secret`) dan `sharedservice-oauth` sebagai `nadi`. Jalankan `make smoke`, lalu di dashboard chat minta model mendaftar perangkat dan membuat satu perangkat. Verifikasi:
  - model memanggil `get_operation_schema` sebelum membuat perangkat;
  - hasil `call_operation` dan riwayat chat tidak memuat `acted_by`.

  Catat hasilnya di tugas ini, lalu bersihkan clone dan proses.

## Workflow follow-up

- Archive change ini setelah semua tugas tercentang dan tes terkait hijau, commit di branch worktree, merge `feat/add-agent-operation-schemas` ke `main`, lalu hapus worktree dan branch-nya. Push hanya bila diminta.
