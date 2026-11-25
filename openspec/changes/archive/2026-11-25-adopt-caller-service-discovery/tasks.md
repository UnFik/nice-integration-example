# Tasks

Perintah verifikasi (dari `agent-backend/`, dengan Go dari mise):
- `heavy go test -race ./... -count=1`
- `heavy go vet ./...`
- `heavy go test ./internal/ -run Arch -count=1` untuk tes arsitektur.

Tes unit tidak boleh butuh NICE yang berjalan. NICE dan `nadi` dipalsukan di `internal/bootstrap/fakes_test.go`.

## 1. Persiapan

- [x] 1.1 Buat worktree `.worktrees/example-adopt-caller-service-discovery` dari `main`, dengan branch `feat/adopt-caller-service-discovery`, lalu pindahkan `openspec/changes/adopt-caller-service-discovery/` ke worktree. Verifikasi: `heavy go test -race ./... -count=1` di `agent-backend/` lulus sebelum ada perubahan.
- [x] 1.2 Jalankan `go get github.com/ABA-Developer/nice-auth-sdk@v0.2.4 && go mod tidy` di `agent-backend/`. Verifikasi: `go.mod` menyebut `v0.2.4`, dan `heavy go build ./...` serta tes yang ada tetap lulus.
- [x] 1.3 Buat tes kecil di `internal/bootstrap` untuk memastikan tiga perilaku go-sdk `v1.8.0` dalam mode `Stateless: true, JSONResponse: true` yang dipakai design (keputusan 4). Verifikasi: tes lulus. Bila salah satu tidak berlaku, hentikan dan bahas ulang design sebelum lanjut.
  - Handler tool yang mengembalikan `CallToolResult{InputRequests, RequestState}` dijawab sebagai `resultType: input_required` ke client yang mengirim `MCP-Protocol-Version: 2026-07-28`.
  - Pengulangan dengan `inputResponses` dan `requestState` sampai ke `req.Params`.
  - `req.Session.InitializeParams().ProtocolVersion` mencerminkan header itu.

## 2. Domain, DTO, dan adapter NICE

- [x] 2.1 Ubah `internal/domain`:
  - `Target` menjadi slug bebas, dan konstanta `Nadi` dihapus;
  - tambah `ValidServiceSlug` (`^[a-z][a-z0-9-]{0,62}$`);
  - tambah `domain.APIError{Status, Code}` dan `domain.ErrInvalidCall` di `exchange_domain.go`.

  Verifikasi: tes tabel `ValidServiceSlug` mencakup slug valid, huruf besar, spasi, `/`, dan string kosong.
- [x] 2.2 Tambah DTO peta dan operasi di `internal/app/call/dto/service_map_dto.go` dan `operation_dto.go`. Ganti `exchange_dto.go` dengan `OperationCall` / `OperationResult` (status dan body). Verifikasi: tes arsitektur lulus.
- [x] 2.3 Tambah `internal/adapter/niceexchange/discovery_client.go` (`Services`, `ServiceOperations`) dan ganti `sender_client.go` dengan `operation_client.go` (`CallOperation`).
  - Petakan SDK → DTO dan galat SDK → domain (`GrantError`, sementara, tidak terjangkau, `APIError`, `ErrInvalidOperationCall`).
  - Body jawaban dibatasi 1 MiB.

  Verifikasi: tes adapter dengan NICE palsu `httptest` mencakup peta multi-halaman, `projects_truncated`, `404 not_found` operasi, `404` peta, parameter path `..` (tanpa exchange), dan `token_stale`.

## 3. Use case discovery dan panggilan

- [x] 3.1 Ganti kontrak `Sender` di `call_contract.go` dengan `Discoverer` dan `Operator`. Tambah `list_services_usecase.go` dan `list_operations_usecase.go`, lalu hapus `execute_call_usecase.go`. Pemetaan galat mengikuti design keputusan 5 (`discovery_unavailable`, `service_not_allowed`, `token_stale_after_upgrade`). Verifikasi: tes use case dengan fake `Discoverer` lulus.
- [x] 3.2 Tambah `call_operation_usecase.go`: validasi argumen, resolusi service, konteks, dan operasi dari peta, lalu cek nama `query` (design keputusan 2). Verifikasi: tes mencakup setiap skenario "Panggilan operasi atas nama user" dan "Konteks tool" di delta spec. Untuk setiap kasus `invalid_argument`, tes memastikan `Operator` tidak dipanggil.
- [x] 3.3 Tambah `internal/app/call/internal/approval/approval_helper.go`: menandatangani dan memeriksa token persetujuan (HMAC-SHA256, digest JSON kanonik, `exp` 5 menit, set nonce sekali pakai), dengan key acak dari konstruktor. Sambungkan ke `call_operation_usecase.go`: operasi sensitif tanpa persetujuan valid menghasilkan "perlu persetujuan" berisi pesan dan state, bukan panggilan. Verifikasi: tes mencakup persetujuan valid sekali pakai (pemakaian kedua ditolak), `sub` lain, argumen lain, operasi lain, kedaluwarsa, tanda tangan diubah, dan `decline`/`cancel`. Untuk setiap penolakan, tes memastikan `Operator` tidak dipanggil.
- [x] 3.4 Ubah `failure.Exchange` dan `failure.Service` agar menerima slug, dan tambah kode `approval_unavailable` dan `not_approved`. Verifikasi: tes memastikan tidak ada token, `requestState`, atau body mentah di `Failure`.

## 4. Tool MCP

- [x] 4.1 Ganti `tools_routes.go`: daftarkan `list_services`, `list_operations`, dan `call_operation` dengan anotasi dari design keputusan 1. Tambah `list_services_handler.go`, `list_operations_handler.go`, dan `call_operation_handler.go`, lalu hapus `list_devices_handler.go` dan `create_device_handler.go`. Verifikasi: `tools_test.go` menunjukkan `tools/list` berisi tepat tiga tool itu.
- [x] 4.2 Tambah `internal/adapter/mcp/internal/approval/elicit_helper.go`:
  - membangun `InputRequests{"approve": ElicitParams}` mode form dengan skema `{approve: boolean}`, dan pesan yang menyebut service, ringkasan, method, path terisi, organisasi, dan project (disaring `failure.safe`, dibatasi panjangnya);
  - membaca `inputResponses`;
  - mengembalikan `approval_unavailable` untuk client dengan protokol < `2026-07-28` atau tanpa elicitation form.

  Verifikasi: tes MCP end-to-end di `internal/bootstrap` dengan NICE dan `nadi` palsu mencakup lima kasus:
  - delete sensitif disetujui: tepat satu DELETE sampai ke `nadi`;
  - delete ditolak: nol request;
  - client protokol lama: `approval_unavailable` dan nol exchange;
  - `requestState` dipakai ulang dengan `id` lain: tidak ada panggilan;
  - operasi GET tanpa elicitation.
- [x] 4.3 Perbarui `call_test.go` dan `parallel_test.go` untuk tool baru. Verifikasi: skenario "Banyak panggilan" (satu exchange `nadi` dan satu exchange API NICE untuk sepuluh panggilan) dan "Dua konteks sekaligus" lulus dengan `-race`.

## 5. Konfigurasi, bootstrap, dan dokumen

- [x] 5.1 Hapus `NADI_URL` dari `config.go`, `.env.example`, `doc.go`, `LogValue`, `bootstrap.go` (`Bases`), dan `README` (id/en). Verifikasi: tes config memastikan `.env.example` tidak memuat `NADI_URL`, README mendokumentasikan tepat variabel `.env.example`, dan agen mulai hanya dengan tiga variabel wajib.
- [x] 5.2 Perbarui `README.md` dan `README.en.md` dengan isi setara:
  - tabel tiga tool;
  - alur discovery;
  - persetujuan operasi sensitif dan protokol MCP yang dibutuhkan;
  - batas set nonce lintas replika;
  - syarat endpoint katalog `https` atau loopback;
  - checklist setup dan uji (langkah `curl` dan MCP Inspector memakai `list_services` → `list_operations` → `call_operation`);
  - versi NICE minimum `235445c`.

  Verifikasi: `docs/readme_test.go` lulus, dan setiap potongan `curl` di README memakai nama tool dan argumen yang ada.
- [x] 5.3 Perbarui `scripts/smoke.sh`: `tools/list` berisi tepat tiga tool, `list_services` memuat `nadi`, `list_operations nadi` memuat operasi `GET /devices`, dan `call_operation` untuk operasi itu menjawab sukses. Tidak ada token yang dicetak. Nomor langkah checklist harus cocok dengan README. Verifikasi: `bash -n scripts/smoke.sh`, lalu smoke terhadap stack dev (lihat 6.1).
- [x] 5.4 Perbarui README akar (id/en): tabel versi SDK per contoh (`agent-backend` `v0.2.4`, yang lain `v0.2.3`) dan revisi NICE minimum `agent-backend`. Perbarui juga kalimat versi SDK di `AGENTS.md`. Verifikasi: tabel di kedua README setara.

## 6. Integrasi

- [x] 6.1 Jalankan NICE `dev` (≥ `235445c`) dan `nadi` lokal dengan endpoint katalog `http://127.0.0.1:8081`. Jalankan `make smoke` dengan token user yang memegang `agen.mcp.use` dan izin `nadi`. Verifikasi: smoke lulus. Catat hasilnya di bagian ini.
- [x] 6.2 Uji manual dengan MCP Inspector, atau `curl` dua putaran dengan header `MCP-Protocol-Version: 2026-07-28`. Verifikasi dan catat hasilnya:
  - buat perangkat lewat `call_operation`, dan perangkat menunjukkan `acted_by` agen di `nadi`;
  - hapus perangkat meminta persetujuan dan berhasil setelah disetujui;
  - penolakan tidak menghapus apa pun.
  - Hasil 6.1 dan 6.2 (2026-11-25), dijalankan di VPS, bukan lokal (pilihan pengguna). Kondisinya: NICE `dev-235445c`, image `agen:discovery-wip` dari worktree, endpoint katalog `nadi` `https://nadi.unfik.my.id`, login `demo@unfik.my.id` lewat `agen-user`, dan `scripts/smoke.sh` dengan `AGENT_URL=https://agen.unfik.my.id`. Smoke lulus: tiga tool, `list_services` memuat `nadi` di 1 project, `list_operations` 5 operasi, dan `call_operation listDevices` HTTP 200. Uji persetujuan lulus:
    - `createDevice` mencatat `created_by` demo dan `acted_by` `agen-backend`;
    - client protokol lama dijawab `approval_unavailable`;
    - putaran pertama `input_required` dengan pesan "Approve a sensitive operation of NADI (nadi): Delete a device. DELETE /devices/<id> in Demo / Demo.";
    - `approve=false` dijawab `not_approved`;
    - state untuk id lain meminta persetujuan baru;
    - setelah disetujui, hapus dijawab 204;
    - state yang dipakai ulang meminta persetujuan baru, dan perangkat hilang dari daftar.
- [x] 6.3 Jalankan `heavy go test -race ./... -count=1`, `heavy go vet ./...`, dan `heavy go build ./...` di `agent-backend/`. Verifikasi: semuanya lulus.

## Workflow follow-up

- Arsipkan change setelah semua tugas tercentang, commit di branch worktree, merge ke `main`, lalu hapus worktree dan branch. Push hanya bila diminta.
- Deploy VPS (di luar repo):
  - naikkan NICE `my-vps` ke `dev` terbaru;
  - ubah endpoint katalog `nadi` ke `https://nadi.unfik.my.id` lewat API, lalu pastikan path operasi `nadi` dan `/healthz` sampai ke `nadi:8081`;
  - hapus `NADI_URL` dari `/opt/nice/compose.examples.yml`;
  - muat image `agen` baru;
  - perbarui `~/.config/nice-vps-agen-smoke.sh` dan salinan smoke di scratchpad;
  - jalankan smoke dan uji hapus dengan persetujuan sebagai `demo@unfik.my.id`.
