# Proposal

## Why

`agent-backend` saat ini hanya server MCP. Untuk mencoba agen, user butuh client MCP (Claude Code, Claude Desktop, atau MCP Inspector) dan harus mengurus token sendiri, sehingga mitra tidak punya contoh yang bisa langsung dibuka di browser. Contoh ini juga belum menunjukkan bagaimana agen AI sungguhan (LLM) memakai tool discovery dan alur persetujuan operasi sensitif yang sudah ada.

## What Changes

- **BREAKING (tata letak repo):** `agent-backend/` dipindah ke `agent/backend/`, mengikuti pola `sharedservice-oauth/` (`backend/` + `web/`). Path modul Go, `cmd/`, Makefile, Dockerfile, README, `AGENTS.md`, README akar, dan tautan di contoh lain ikut diperbarui. Perilaku MCP di `/mcp` tidak berubah.
- Folder baru `agent/web/`: dashboard chat React 19 + Vite yang hanya berbicara ke backend di origin yang sama. UI tidak memegang token, tidak memanggil NICE, dan tidak memanggil LLM.
- Backend agen menjadi backend-for-frontend juga:
  - `/auth/*` untuk login user ke NICE sebagai client confidential yang diikat ke `agen`;
  - sesi cookie di sisi server, CSRF, konteks org dan project, pindah konteks, dan keluar, dengan pola yang sama seperti `sharedservice-oauth/backend`.
- API chat `/chat/*`: daftar, buat, ganti judul, dan hapus percakapan; kirim pesan; jawab permintaan persetujuan.
  - Satu giliran chat menjalankan loop LLM yang memanggil tiga tool yang sama dengan MCP (`list_services`, `list_operations`, `call_operation`) lewat use case di dalam proses.
  - Tool dijalankan atas nama user lewat token sesi dan token exchange yang sudah ada.
- LLM provider-agnostik. Ada satu kontrak model di lapisan app dan dua adapter: Anthropic Messages API (default `claude-sonnet-5-5`) dan API Chat Completions yang kompatibel OpenAI (OpenAI, Ollama, vLLM, dan sejenisnya). Pilihan provider, model, base URL, dan API key diatur lewat env. API key hanya ada di backend.
- Persetujuan operasi sensitif di chat:
  - `call_operation` yang butuh persetujuan menghentikan giliran dan menyimpan panggilan yang tertunda;
  - UI menampilkan pesan persetujuan yang dibangun dari katalog, tidak dari teks LLM;
  - jawaban Setujui/Tolak melanjutkan giliran memakai state persetujuan sekali pakai yang sudah ada.
- Riwayat percakapan disimpan di PostgreSQL per user (`sub`), dengan migrasi yang diterapkan saat start. Tanpa `DATABASE_URL` riwayat disimpan di memori, sama seperti perangkat di `nadi`, untuk dev dan tes. Token tidak pernah disimpan di riwayat.
- Image container: satu image membangun UI lalu backend, dan menyajikan dashboard di `/` serta MCP di `/mcp`. Karena itu `https://agent.unfik.my.id` tetap satu host di tunnel.

## Capabilities

### New Capabilities
- `agent-chat-example`: dashboard chat agen, yaitu login BFF ke NICE lewat backend agen, percakapan per user di PostgreSQL, loop LLM provider-agnostik yang memakai tool agen, dan persetujuan operasi sensitif di UI.

### Modified Capabilities
- `agent-backend-example`: lokasi contoh pindah ke `agent/backend/`. Panduan setup dan uji ditambah client confidential untuk login dan variabel LLM. Image container ikut menyajikan UI. Persyaratan MCP lainnya tetap.

## Impact

- **Kode:**
  - `agent-backend/` dipindah ke `agent/backend/` dan path modul menjadi `github.com/nice-platform/example/agent/backend`.
  - Paket baru di backend: sesi, chat, model LLM, adapter Anthropic dan OpenAI-compatible, repositori memori dan PostgreSQL, serta `migrations/`.
  - Folder baru `agent/web/`.
- **Dependensi:** `anthropic-sdk-go` dan `openai-go` untuk adapter LLM, driver PostgreSQL yang sama dengan `nadi`, serta React 19, Vite, dan Vitest di `agent/web`.
- **NICE (setup, bukan kode):** client confidential `agen-backend` diberi redirect `PUBLIC_URL/auth/callback`, post-logout URI, dan grant `refresh_token`. Client yang sama tetap menjadi aktor token exchange. NICE menerima subject token ber-`aud=agen` milik client itu sendiri (`SubjectToken.ForService`), jadi tidak perlu client kedua.
- **Deploy VPS:** image `agen` dibangun dari `agent/`, ditambah env LLM dan `DATABASE_URL` (database baru di PostgreSQL VPS). Redirect client di NICE VPS diperbarui lewat `~/.config/nice-vps-register.sh` dengan token admin di `~/.config/nice-vps-admin.json` (scope `openid profile offline_access`).
- **Dokumen:** README akar (id/en), `AGENTS.md`, README `agent/` (id/en), `sharedservice-oauth` dan `shared-service-apikey` (tautan), serta `compose.examples.yml` di VPS.
