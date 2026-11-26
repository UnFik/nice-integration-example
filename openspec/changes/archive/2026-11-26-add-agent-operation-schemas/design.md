# Design

## Context

Lihat `proposal.md` untuk motivasi. Keadaan `agent/backend` sekarang:

- **Tool dan model.** Tabel `tools` di `internal/adapter/mcp/tools_routes.go` adalah satu sumber untuk server MCP dan model chat (`ToolSpecs`). Ada tiga tool: `list_services`, `list_operations`, dan `call_operation`. Model memakai `call_operation` dengan `path_params`, `query`, dan `body` bebas.
- **Kontrak use case.** Use case `internal/app/call` bergantung pada `Discoverer` (`Services`, `Operations`) dan `Operator` (`CallOperation`). Keduanya diisi `internal/adapter/niceexchange`, satu-satunya tempat SDK dipakai.
- **Jalur `call_operation`.** Use case membaca peta dan operasi dari NICE di setiap panggilan, lalu `Operator.CallOperation`. Jawaban 2xx di-decode menjadi `CallView.Body`; jawaban lain menjadi `failure.Service`, yang hanya membaca `error` dan `error_description`.
- **SDK v0.2.5** menyediakan `ServiceOperationsIfChanged`, `OperationSchemas`, `OperationSchema.ResponseSchema`, dan `RedactSecrets`.

## Goals / Non-Goals

**Goals:**

- Model melihat arti operasi dan field tanpa memuat skema semua operasi.
- Properti `x-secret` tidak pernah sampai ke model atau riwayat chat bila NICE mengenalnya.
- Contoh tetap satu-satunya pemakai SDK di adapter, dan use case tetap bebas dari tipe SDK.

**Non-Goals:**

- Satu tool per operasi.
- Validasi `body` terhadap `input_schema` di agen. Service tetap yang memvalidasi.
- Cache peta service (`list_services`).

## Decisions

### DTO dan kontrak

- **`calldto.Operation`** mendapat `Description`.
- **`calldto.RequestField`** mendapat `Description`, `Format`, `Enum []any`, dan `EnumTruncated`. Tag JSON-nya memakai `omitempty`, sehingga `list_operations` tidak memuat isian kosong.
- **`calldto.OperationView`** mendapat `Description`.
- **`Discoverer` mendapat dua method:**
  - `OperationSchemas(ctx, token, slug string, keys []string) (calldto.OperationSchemas, error)`, dengan `calldto.OperationSchema{Key, InputSchema, Responses json.RawMessage, Truncated}` dan `Missing []string`;
  - `Redact(schema calldto.OperationSchema, status int, body []byte) ([]byte, error)`, yang membungkus `ResponseSchema` dan `RedactSecrets`.
  `Redact` diletakkan di adapter supaya use case tidak mengimpor SDK.
- **`Operations(ctx, token, slug)`** mendapat parameter `user string` (`Caller.Subject`) untuk kunci cache.

### Cache daftar operasi

- **Tempat dan kunci.** Cache di `niceexchange.Client`, dengan kunci `user + "\x00" + slug`, berisi `credentials.ServiceOperations` terakhir dan `ETag`-nya.
- **Batas dan pengusiran.** Maksimal 1000 entri, dengan pengusiran LRU sederhana (map + list) di bawah mutex. Tidak ada TTL, karena setiap pembacaan tetap bertanya ke NICE.
- **Pembacaan.** Setiap `Operations` memanggil `ServiceOperationsIfChanged` dengan `ETag` tersimpan:
  - `changed=false` memakai simpanan;
  - `changed=true` mengganti simpanan;
  - galat menghapus entri, sehingga daftar lama tidak dipakai setelah NICE menolak, mis. 404 karena izin dicabut.
- **Alternatif yang ditolak:**
  - kunci per token: token berganti setiap refresh, sehingga cache hampir tidak pernah kena;
  - TTL tanpa revalidasi: izin user bisa berubah, dan agen tidak boleh menawarkan operasi yang sudah dicabut.

### Tool `get_operation_schema`

- **Input:** `service` (slug) dan `operations` (`[]string`, 1–10). Batas 10 menjaga hasil tool tetap kecil di konteks model, karena satu skema bisa sampai 64 KiB. SDK sendiri memecah per 20.
- **Use case `GetOperationSchemas`.** Validasi slug dan jumlah key, lalu `Discoverer.OperationSchemas`. Galat discovery memakai `failure.Discovery` yang sudah ada. 404 dari NICE lama dijawab dengan kode `schemas_unavailable` dan pesan bahwa NICE belum menyajikan skema operasi.
- **Hasil:** `{"service", "schemas": [{"key", "input_schema", "responses", "truncated"}], "missing": []}`. `input_schema` dan `responses` dikirim sebagai JSON mentah (`json.RawMessage`) apa adanya.
- **Deskripsi tool** meminta model memanggilnya sebelum `call_operation` yang memakai `body` atau parameter yang belum jelas, dan memberi tahu bahwa `truncated` berarti skema tidak lengkap.
- **Anotasi** `readOnly`, sama dengan dua tool baca lain. `newServer` mendaftarkannya, dan `ToolSpecs` otomatis mengikutinya.

### Pembersihan rahasia di `call_operation`

- **Skema dibaca sebelum panggilan.** Setelah operasi ditemukan dan sebelum persetujuan sensitif serta `Operator.CallOperation`, use case memanggil `OperationSchemas(token, slug, [key])`:
  - galat sementara atau tak terjangkau: kembalikan `failure` dan jangan kirim panggilan. Lebih baik gagal sebelum efek samping daripada mengirim jawaban yang tidak bisa dibersihkan;
  - `APIError` 404 (NICE tanpa skema): lanjut tanpa skema, dan jawaban tidak dibersihkan;
  - key di `missing`: diperlakukan sebagai operasi yang tidak boleh dipanggil (`invalid_argument`), sama dengan key yang tidak ada di daftar.
- **Jawaban dibersihkan.** Setelah jawaban diterima, `Discoverer.Redact(schema, status, body)` berjalan sebelum decode 2xx dan sebelum `failure.Service`:
  - galat `Redact` (body bukan JSON) pada 2xx tetap menjadi `bad_answer`, seperti sekarang;
  - pada jawaban galat, body asli tidak diteruskan dan `failure.Service` menerima body kosong, sehingga pesannya memakai status saja.
- **Biaya.** Satu permintaan skema tambahan per `call_operation`, dengan token terdelegasi yang sama. Alternatif cache skema per digest ditolak untuk contoh ini: kodenya lebih panjang, padahal biayanya kecil.

### Versi dan dokumen

- `go get github.com/ABA-Developer/nice-auth-sdk@v0.2.5 && go mod tidy`.
- `README.md` dan `README.en.md` akar: baris `agent/backend` memakai SDK `v0.2.5`, dengan NICE minimum revisi `dev` yang memuat `add-operation-schemas`. `AGENTS.md` mengikuti.
- `agent/README.md` dan `README.en.md`: tabel tool memuat `get_operation_schema`, ditambah paragraf soal pembersihan `x-secret` dan perilakunya terhadap NICE lama.
- `scripts/smoke.sh`:
  - daftar tool yang diharapkan memuat `get_operation_schema`;
  - satu langkah memanggilnya untuk key `GET /devices`.

  Nomor langkah checklist di README tidak berubah.

## Risks / Trade-offs

- **[Satu permintaan NICE tambahan per `call_operation`]** → Murah (≤ 64 KiB, gzip), dan menutup kebocoran rahasia. Dicatat di README.
- **[Cache per user di memori satu proses]** → Hanya menghemat ukuran jawaban. Benar di banyak replika, karena setiap pembacaan tetap bertanya ke NICE.
- **[NICE tanpa skema meneruskan jawaban utuh]** → Tidak ada penanda `x-secret` yang hilang, karena dokumen belum dibaca NICE. README menyebut versi NICE minimum.
- **[Model tidak memanggil `get_operation_schema`]** → `call_operation` tetap berjalan seperti sekarang. Service tetap memvalidasi body.

## Migration Plan

- Rilis contoh berikutnya bersama dokumen versi.
- Rollback: kembali ke commit sebelumnya. Tidak ada data yang tersimpan, karena cache hanya ada di memori.
