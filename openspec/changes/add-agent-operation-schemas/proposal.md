# Proposal

## Why

`agent/backend` memakai `nice-auth-sdk` v0.2.4. Sejak NICE `add-operation-schemas` dan SDK v0.2.5, NICE memberi isian yang membantu model memakai operasi dengan benar, tetapi agen belum memakainya:
- `list_operations` hanya memberi nama dan tipe field request, tanpa deskripsi operasi, deskripsi field, `format`, dan pilihan nilai (`enum`);
- model tidak bisa melihat bentuk body bersarang dan bentuk jawaban, sehingga menebak isi `body` dan arti data;
- jawaban service sampai ke model dan riwayat chat apa adanya, termasuk properti yang dokumen service tandai `x-secret`;
- daftar operasi diambil utuh di setiap tool call.

Contoh ini menjadi rujukan mitra (termasuk NAIA) untuk alur agen, jadi ia perlu menunjukkan pemakaian yang benar.

## What Changes

- **SDK naik ke v0.2.5.** Paginasi daftar operasi dan gzip ikut tanpa kode tambahan.
- **`list_operations` lebih kaya (aditif).**
  - Operasi membawa `description`.
  - Field request membawa `description`, `format`, `enum`, dan `enum_truncated`.
  - Isian yang tidak dinyatakan dokumen tidak muncul.
- **Tool baru `get_operation_schema`** (hanya baca). Input: `service` dan `operations` (1–10 key dari `list_operations`). Untuk tiap key, tool mengembalikan `input_schema` dan `responses` dari NICE, `truncated`, serta `missing` untuk key yang tidak boleh dipanggil user. Deskripsi tool menyarankan model memanggilnya sebelum `call_operation` yang memakai `body`.
- **`call_operation` membuang field rahasia.**
  - Sebelum memanggil service, agen membaca skema operasi itu.
  - Jawaban service, juga jawaban galat yang diteruskan ke model, dibersihkan dari properti `x-secret` menurut skema status jawabannya, sebelum sampai ke model dan riwayat percakapan.
  - **Bila skema tidak bisa dibaca karena galat sementara, panggilan tidak dikirim.** Bila NICE belum menyajikan skema (`404`), panggilan tetap berjalan tanpa pembersihan, karena dokumen belum menandai apa pun.
- **Cache daftar operasi per user.** Adapter menyimpan daftar operasi dan `ETag`-nya per (user, service) di memori dengan batas jumlah entri, dan memakai `ServiceOperationsIfChanged`. Daftar tidak pernah dipakai tanpa diperiksa ke NICE (304 menghemat ukuran jawaban, bukan pemeriksaan izin).
- **Dokumen.**
  - `agent/README.md` dan `README.en.md`: tool baru, pembersihan rahasia, dan versi NICE/SDK.
  - Tabel versi di `README.md`/`README.en.md` akar dan `AGENTS.md` (`agent/backend` di SDK `v0.2.5`).

## Capabilities

### New Capabilities

Tidak ada.

### Modified Capabilities

- `agent-backend-example`:
  - discovery memakai SDK v0.2.5, isian operasi yang lebih kaya, dan cache `ETag`;
  - tool `get_operation_schema`;
  - pembuangan field `x-secret` dari jawaban `call_operation`.

## Impact

- **`agent/backend`:**
  - `go.mod`/`go.sum`;
  - `internal/adapter/niceexchange` (daftar, cache, skema, pembersihan);
  - `internal/app/call` (DTO, kontrak `Discoverer`, use case `ListOperations`, `CallOperation`, dan `GetOperationSchemas`);
  - `internal/adapter/mcp` (tabel tool dan handler baru);
  - tes unit dan `scripts/smoke.sh` bila menyentuh tool baru.
- **Dashboard chat.** Memakai tabel tool yang sama, jadi model chat ikut mendapat `get_operation_schema` tanpa perubahan `agent/web`.
- **NICE minimum.** Butuh NICE dengan `add-operation-schemas`. Paginasi dari `add-sdk-operation-schemas` opsional, karena SDK punya fallback. Terhadap NICE lebih lama, isian baru kosong dan `get_operation_schema` menjawab galat `404` yang jelas.
- **Di luar cakupan:**
  - mengubah operasi menjadi tool satu per satu;
  - validasi argumen terhadap `input_schema`;
  - contoh Go lain (`sharedservice-oauth`, `service-account-client`), yang tidak memakai discovery.
