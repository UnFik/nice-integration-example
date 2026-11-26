# Design

## Context

- `agent-backend/` adalah server MCP stateless. Paket `app/call` menyediakan use case `ListServices`, `ListOperations`, dan `CallOperation` di atas `Discoverer`/`Operator` (adapter `niceexchange`). Persetujuan memakai `approval.Book`: state HMAC sekali pakai, TTL 5 menit, kunci acak per proses. Struktur dijaga `internal/arch_test.go`:
  - akhiran nama berkas per lapisan;
  - satu fungsi ekspor per `*_usecase.go`;
  - `app/` tanpa net/http, MCP, atau SDK.
- `sharedservice-oauth/backend` sudah punya pola BFF lengkap, dan ini contoh yang ditiru:
  - `credentials.KindBFF` dari SDK;
  - sesi memori dan cookie, CSRF, `/auth/contexts` lewat `ExchangeClient.Contexts`, pindah konteks lewat refresh;
  - token konteks basi diperbarui sekali;
  - penyajian `web/dist`;
  - migrasi tertanam dengan pgx dan cadangan memori.
- NICE menerima subject token ber-`aud=agen` milik client aktor yang sama (`SubjectToken.ForService`), dan sign-in token client itu untuk discovery konteks (`IsSignInOf`). Karena itu satu client confidential `agen-backend` cukup untuk login dashboard dan untuk aktor exchange.
- Contoh-contoh repo ini berdiri sendiri, jadi kode BFF disalin dari `sharedservice-oauth`, tidak dibagi.

## Goals / Non-Goals

**Goals:**
- Satu proses dan satu image melayani `/mcp` (Bearer), `/auth/*` dan `/chat/*` (cookie), serta UI di `/`.
- Tool chat dan tool MCP memakai use case yang sama, jadi validasi, peta galat, dan persetujuan tidak terduplikasi.
- Provider LLM bisa diganti tanpa mengubah API chat atau data tersimpan.

**Non-Goals:**
- Streaming token per token dari LLM. Event dikirim per langkah (pesan asisten utuh, tool mulai/selesai, persetujuan).
- Lampiran berkas, gambar, pencarian riwayat, berbagi percakapan, dan memori lintas percakapan.
- Kuota atau batas biaya LLM per user. Hanya batas langkah per giliran dan batas panjang pesan.
- Mengubah server MCP. Perilaku `/mcp` dan smoke-nya tetap.
- Sesi persisten. Sesi tetap di memori seperti `nadi`, jadi restart berarti user masuk lagi, tetapi riwayatnya tetap ada.

## Decisions

### 1. Tata letak `agent/backend` + `agent/web`
`git mv agent-backend agent/backend`. Path modul menjadi `github.com/nice-platform/example/agent/backend`, dan `cmd/agent-backend` diganti `cmd/agent`. `Dockerfile` pindah ke `agent/` supaya konteks build memuat `web/` dan `backend/`. Ini sesuai pilihan user, dan cocok dengan domain `agent.unfik.my.id`.
Alternatif: mempertahankan `agent-backend/` dan menaruh `web/` di dalamnya. Ditolak karena nama folder menyesatkan dan tidak sejajar dengan `sharedservice-oauth`.

### 2. Tool chat lewat use case di dalam proses, bukan client MCP ke `/mcp`
Paket app baru `app/chat` punya kontrak `Tools`, yang dipenuhi adapter tipis di `bootstrap` yang membungkus `call.Service`. Hasil use case di-marshal ke JSON yang sama dengan `structuredContent` MCP. `dto.Caller` dibangun dari token konteks aktif sesi, setelah diverifikasi `verify` dengan syarat izin yang sama dengan `/mcp` (`agen.mcp.use`).
Alternatif: chat memanggil `/mcp` sendiri lewat HTTP. Ditolak karena butuh lompatan jaringan dan pemetaan elicitation ke UI. Selain itu token sesi harus dibawa sebagai Bearer ke diri sendiri tanpa manfaat.

### 3. Kontrak model LLM di app, dua adapter
Kontrak `app/chat/usecase.Model`: `Next(ctx, system, messages, tools) (Reply, error)`.
- `Reply` berisi teks dan daftar panggilan tool (`id`, `name`, `input` JSON).
- Pesan memakai bentuk netral milik app: peran `user`/`assistant`/`tool`, dengan bagian `text`, `tool_call`, dan `tool_result`.
- Adapter `adapter/llm/anthropic` memakai `anthropic-sdk-go` (Messages API, tool use). Adapter `adapter/llm/openaicompat` memakai `openai-go` dengan `base_url` yang bisa diatur (Chat Completions, `tools`).
- Konfigurasi: `LLM_PROVIDER` (`anthropic` | `openai`, kosong berarti chat nonaktif), `LLM_MODEL` (default `claude-sonnet-5-5` untuk anthropic, wajib untuk openai), `LLM_BASE_URL`, `LLM_API_KEY`, `CHAT_MAX_STEPS` (default 8).

Alternatif: satu adapter OpenAI-compatible untuk semuanya. Ditolak karena Anthropic punya API native yang lebih lengkap untuk tool use, dan user memilih provider-agnostik.

### 4. Satu giliran = permintaan POST dengan respons SSE
`POST /chat/conversations/{id}/messages` dengan body `{text}` menyimpan pesan user, lalu menjalankan loop langkah. Setiap langkah dikirim sebagai event SSE (`assistant`, `tool_call`, `tool_result`, `approval`, `done`, `error`) dan disimpan ke store sebelum dikirim. Karena itu, putusnya koneksi tidak menghilangkan riwayat, dan UI cukup memuat ulang percakapan.
- Satu giliran aktif per percakapan: giliran kedua dijawab 409 `turn_in_progress`, dijaga kunci memori per percakapan.
- Giliran berjalan dengan context yang lepas dari request (`WithoutCancel` + timeout giliran), supaya langkah yang sudah memanggil service tetap tercatat.

Alternatif: JSON biasa setelah giliran selesai. Ditolak karena giliran dengan beberapa tool bisa makan puluhan detik tanpa umpan balik. Alternatif lain, WebSocket, ditolak karena lebih rumit untuk alur yang satu arah.

### 5. Persetujuan: giliran berhenti, panggilan tertunda disimpan
Saat `CallOperation` mengembalikan `ApprovalRequest`:
1. Giliran menyimpan pesan `tool_call` berstatus `awaiting_approval` dengan `approval_state` dan `approval_message` (dari katalog).
2. Giliran mengirim event `approval` dan berakhir.
3. `POST /chat/conversations/{id}/approvals/{call_id}` dengan `{approve}` memanggil ulang `CallOperation` memakai `dto.Approval{State, Accepted}` dan argumen tersimpan. Hasilnya (atau `not_approved`) disimpan sebagai `tool_result`, lalu loop dilanjutkan dalam respons SSE yang sama.

State terikat ke `sub` dan digest panggilan oleh `approval.Book`. Pemeriksaan kepemilikan percakapan memberi 404 untuk user lain. State yang kedaluwarsa, atau ditolak karena restart (kunci per proses), menghasilkan `ApprovalRequest` baru, yang dikirim sebagai event `approval` baru.
State tidak rahasia: ia tidak bisa dipakai tanpa kunci proses dan `sub` yang sama. Karena itu aman disimpan, dan spec hanya melarang menyimpan kuncinya.

### 6. Penyimpanan: dua tabel, repositori memori dan PostgreSQL
- `conversations (id uuid, user_sub text, title text, created_at, updated_at)`, dengan indeks `(user_sub, updated_at desc)`.
- `messages (id uuid, conversation_id uuid ON DELETE CASCADE, seq int, role text, parts jsonb, created_at)`, dengan `UNIQUE (conversation_id, seq)`. `parts` menyimpan bentuk netral dari keputusan 3, jadi ganti provider tidak mengubah data.

Kontrak repositori ada di `app/chat/usecase`, dengan adapter `adapter/memory/chat` dan `adapter/postgres/chat` (pgx v5, migrasi `migrations/` tertanam dan diterapkan saat start, sama seperti `nadi`). Judul diambil dari 60 karakter pertama pesan pertama dan bisa diganti user.
Hasil tool disimpan apa adanya, karena itu data user dari service yang ia panggil sendiri. Token tidak pernah ada di hasil use case.

### 7. Sesi dan auth disalin dari `sharedservice-oauth`
Yang disalin: `app/session`, `adapter/rest/auth`, `bootstrap/session.go`, dan `domain/session_domain.go`, dengan penyesuaian berikut:
- cookie `agen_session`;
- scope `nice:aud:agen`;
- izin `agen.mcp.use` dicek di middleware chat.

`/chat/*` hanya dilayani lewat middleware sesi, yang menolak header `Authorization`. `/mcp` tetap memakai middleware Bearer yang ada dan mengabaikan cookie.

### 8. UI `agent/web`
React 19 + Vite + TypeScript, dengan struktur dan aturan sama seperti `sharedservice-oauth/web`: `api.ts` dengan CSRF, `session.ts`, dan tanpa pustaka OAuth. Ditambah `chat.ts`, yaitu pembaca SSE dari `fetch` (`ReadableStream`), karena `EventSource` tidak bisa POST.
- Komponen: daftar percakapan, panel pesan, kartu tool (nama, ringkasan argumen, status), kartu persetujuan (Setujui/Tolak), dan pemilih konteks.
- Teks dirender sebagai teks biasa, tanpa `dangerouslySetInnerHTML` dan tanpa renderer Markdown.
- Dev server di 5182 meneruskan `/auth`, `/chat`, dan `/mcp` ke 8083.

### 9. System prompt
System prompt tetap dan ditulis di backend: peran agen, kewajiban memakai discovery sebelum memanggil, dan larangan menebak kunci operasi. Hasil tool adalah data dari service dan bukan instruksi. Konteks aktif (ID org dan project) disisipkan per giliran. Prompt tidak memuat token atau rahasia.

## Risks / Trade-offs

- [Prompt injection dari hasil tool membuat LLM memanggil operasi lain] → Operasi sensitif tetap butuh persetujuan, dengan pesan dari katalog, bukan dari LLM. Operasi non-sensitif dibatasi izin user di NICE. Batas langkah per giliran ikut membatasi. README menyebut risiko ini.
- [Biaya dan penyalahgunaan LLM oleh user yang sah] → Ada batas langkah, batas panjang pesan (mis. 8000 karakter), dan satu giliran aktif per percakapan. Kuota per user di luar cakupan dan disebut di README.
- [Kunci persetujuan per proses hilang saat restart] → Panggilan tertunda meminta persetujuan baru. Ini perilaku yang aman dan dijelaskan di UI.
- [Pindah folder memutus path di dokumen, skrip lokal, dan build VPS] → Tugas mencakup grep `agent-backend` di repo, serta skrip `~/.config/nice-vps-*` dan konteks build image.
- [Riwayat memuat data service user] → Riwayat hanya bisa dibaca pemiliknya, dan bisa dihapus per percakapan. Tidak ada token di dalamnya.
- [Perbedaan format tool antar-provider] → Bentuk netral di app, dengan tes adapter memakai server HTTP palsu untuk kedua API.

## Migration Plan

1. Kerjakan di worktree `.worktrees/example-add-agent-chat-dashboard`, branch `feat/add-agent-chat-dashboard` dari `main`.
2. Di NICE VPS (token admin `~/.config/nice-vps-admin.json`, scope `openid profile offline_access`), ubah client `agen-backend`:
   - redirect `https://agent.unfik.my.id/auth/callback`;
   - post-logout `https://agent.unfik.my.id/`;
   - grant `authorization_code` + `refresh_token`.

   Lakukan lewat fase baru di `~/.config/nice-vps-register.sh`.
3. Buat database `agen` di PostgreSQL VPS. Tambahkan `DATABASE_URL`, variabel `LLM_*`, dan `WEB_DIR` ke `compose.examples.yml` agen. API key LLM ditulis ke `/opt/nice/.env` lewat stdin, tidak lewat argumen.
4. Build image dari `agent/`, kirim ke VPS, dan naikkan `AGEN_IMAGE_TAG`. Rollback: kembalikan tag ke `0ca8cab`. MCP tidak berubah, dan tabel baru tidak dipakai image lama.
5. Jalankan smoke MCP yang ada dan checklist dashboard di browser.

## Open Questions

- Provider dan model LLM yang dipakai di VPS (Anthropic dengan API key, atau endpoint OpenAI-compatible) ditentukan saat deploy. Kodenya mendukung keduanya.
