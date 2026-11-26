# Tasks

## 1. Pindah ke `agent/backend`

- [x] 1.1 Pindahkan folder dengan `git mv agent-backend agent/backend`, ganti path modul ke `github.com/nice-platform/example/agent/backend` dan `cmd/agent-backend` ke `cmd/agent`, lalu perbarui import, Makefile, dan `internal/arch_test.go`. Selesai bila `heavy go test -race ./... -count=1` di `agent/backend` hijau dan `heavy go vet ./...` bersih.
- [x] 1.2 Pindahkan `Dockerfile` ke `agent/` dengan konteks build `agent/`. Untuk sementara image masih tanpa UI. Selesai bila `heavy docker build -t agen:test agent/` berhasil dan container menjawab `/healthz`.
- [x] 1.3 Ganti semua rujukan `agent-backend` di README akar (id/en), `AGENTS.md`, README `agent/backend` (id/en), README `sharedservice-oauth`, `sharedservice-oauth/backend`, `shared-service-apikey`, dan `service-account-client`. Tambahkan `agent/README.md` dan `README.en.md` ringkas yang menunjuk `backend/` dan `web/`. Delta spec skenario "Folder lama tidak ada": `grep -rn "agent-backend" --exclude-dir=openspec --exclude-dir=node_modules .` tidak menemukan apa pun selain nama binary atau image yang sengaja dipertahankan.

## 2. Sesi dan login dashboard (BFF)

- [x] 2.1 Salin dari `sharedservice-oauth/backend`: `domain/session_domain.go`, `app/session` (dto, contract, usecase), adapter sesi memori, `adapter/rest/auth` (login, callback, session, contexts, context, logout, middleware sesi, CSRF), dan `bootstrap/session.go`. Sesuaikan cookie menjadi `agen_session`, scope menjadi `nice:aud:agen`, dan izin menjadi `agen.mcp.use`. Tambahkan env `PUBLIC_URL`, `POST_LOGOUT_URL`, `SESSION_IDLE_TIMEOUT`, dan `WEB_DIR` ke config dan `.env.example`. Selesai bila tes auth yang disalin, dengan NICE palsu, hijau untuk: login, callback dengan state salah, CSRF 403, pindah konteks, logout, dan `session_ended`.
- [x] 2.2 Pasang rute `/auth/*` dan penyaji `web/dist` di `bootstrap`, dan pastikan `/mcp` tetap hanya menerima Bearer. Selesai bila tes bootstrap membuktikan dua hal: `/mcp` dengan cookie sesi tanpa `Authorization` dijawab 401 berisi `resource_metadata`, dan `/chat/conversations` dengan Bearer tanpa cookie dijawab 401. Tes MCP yang ada tetap hijau.
- [x] 2.3 Perbarui checklist setup README `agent/backend` (id/en) untuk client `agen-backend`: redirect `PUBLIC_URL/auth/callback`, post-logout, dan grant `refresh_token`. Perbarui juga tabel variabel. Selesai bila tes config tetap mencocokkan README dengan `.env.example`.

## 3. Penyimpanan percakapan

- [x] 3.1 Buat `domain/chat_domain.go` (percakapan, pesan, bagian `text`/`tool_call`/`tool_result`, status `awaiting_approval`, batas judul dan panjang pesan), DTO `app/chat/dto`, dan kontrak repositori di `app/chat/usecase/chat_contract.go`. Selesai bila tes domain untuk validasi judul dan panjang pesan hijau, dan `arch_test` hijau.
- [x] 3.2 Buat repositori memori `adapter/memory/chat` dengan operasi: list per `sub` terurut `updated_at desc`, get dengan pemilik, create, rename, delete berikut pesannya, append pesan, dan update status panggilan tertunda. Selesai bila tes repositori memori hijau, termasuk akses ke percakapan user lain yang dijawab not found.
- [x] 3.3 Buat migrasi `migrations/000001_create_conversations_messages` (up/down) beserta `embed.go`, repositori `adapter/postgres/chat` (pgx v5), dan penerapan migrasi saat start bila `DATABASE_URL` diisi. Selesai bila tes repositori yang sama dijalankan terhadap PostgreSQL saat `TEST_DATABASE_URL` diisi (database sementara milik tes), dan dilewati bila tidak. Uji juga riwayat yang masih terbaca setelah repositori dibuat ulang, dan tidak adanya string `eyJ` di baris tersimpan.
- [x] 3.4 Dokumentasikan `DATABASE_URL` dan skema singkatnya di README (id/en) dan `.env.example`. Selesai bila tes config hijau.

## 4. Model LLM provider-agnostik

- [x] 4.1 Buat kontrak `Model` di `app/chat/usecase` dengan bentuk pesan netral dan definisi tool (nama, deskripsi, JSON Schema input yang sama dengan tool MCP). Ambil schema dari satu sumber yang juga dipakai `adapter/mcp/tools_routes.go`. Selesai bila tes memastikan schema chat sama dengan schema yang diumumkan `tools/list` MCP.
- [x] 4.2 Buat adapter `adapter/llm/anthropic` (`anthropic-sdk-go`, Messages API, tool use). Selesai bila tes dengan server HTTP palsu memastikan: request memuat system, pesan, dan tool; balasan `tool_use` dipetakan ke panggilan tool; `tool_result` dikirim balik dengan id yang sama; galat 429/5xx dipetakan ke galat sementara tanpa membocorkan API key.
- [x] 4.3 Buat adapter `adapter/llm/openaicompat` (`openai-go`, Chat Completions, `tools`, `base_url` bisa diatur). Selesai bila tes server palsu yang setara dengan 4.2 hijau, termasuk `tool_calls` dengan argumen string JSON.
- [x] 4.4 Tambahkan config `LLM_PROVIDER`, `LLM_MODEL` (default `claude-sonnet-5-5` untuk anthropic), `LLM_BASE_URL`, `LLM_API_KEY`, dan `CHAT_MAX_STEPS` (default 8), lalu wiring di `bootstrap`. Selesai bila tes config membuktikan: provider kosong berarti chat nonaktif sementara `/mcp` jalan; provider tak dikenal ditolak saat start; `LLM_API_KEY` tidak muncul di `String()` atau log. README (id/en) dan `.env.example` ikut diperbarui.

## 5. Giliran chat dan persetujuan

- [x] 5.1 Buat adapter tool di `bootstrap` yang membungkus `call.Service` (`ListServices`, `ListOperations`, `CallOperation`) untuk kontrak `Tools` di `app/chat`, dengan hasil JSON sama dengan `structuredContent` MCP dan galat lewat peta failure yang sama. Selesai bila tes dengan NICE dan `nadi` palsu (fakes yang sudah ada) hijau untuk ketiga tool.
- [x] 5.2 Buat use case giliran (`run_turn_usecase.go`):
  - simpan pesan user dan susun system prompt dengan konteks aktif;
  - jalankan loop `Model.Next` lalu eksekusi tool, sampai jawaban teks atau batas langkah;
  - simpan setiap langkah sebelum event dikirim;
  - kunci satu giliran per percakapan.

  Selesai bila tes dengan model palsu hijau untuk skenario: daftar perangkat, batas langkah, tool gagal (`service_not_allowed`), dan giliran kedua serentak dijawab `turn_in_progress`.
- [x] 5.3 Tangani `ApprovalRequest` di giliran: simpan panggilan `awaiting_approval` dengan state dan pesan katalog, kirim event `approval`, lalu hentikan giliran. Tambahkan use case `answer_approval_usecase.go` yang memanggil ulang `CallOperation` dengan `dto.Approval`, menyimpan hasil atau `not_approved`, dan melanjutkan loop. Selesai bila tes hijau untuk: setuju (delete terkirim sekali ke `nadi` palsu), tolak (tidak ada exchange untuk `nadi`), state kedaluwarsa (persetujuan baru diminta), dan jawaban dari user lain (404, panggilan tetap tertunda).
- [x] 5.4 Buat use case percakapan: list, get, create, rename, delete. Selesai bila tes use case hijau, termasuk 404 untuk pemilik lain.
- [x] 5.5 Buat handler REST `adapter/rest/chat`:
  - `GET/POST /chat/conversations`;
  - `GET/PATCH/DELETE /chat/conversations/{id}`;
  - `POST /chat/conversations/{id}/messages` (SSE);
  - `POST /chat/conversations/{id}/approvals/{call_id}` (SSE).

  Semua di balik middleware sesi, CSRF, dan `agen.mcp.use`, dengan context giliran yang lepas dari request. Selesai bila tes HTTP hijau untuk: urutan event SSE, 409 `turn_in_progress`, 503 `chat_not_configured` tanpa provider, 413 untuk pesan terlalu panjang, dan tidak adanya token di body respons.
- [x] 5.6 Tambahkan bagian "Dashboard chat" di README `agent/backend` (id/en): alur giliran, persetujuan, batas langkah, risiko prompt injection, dan tidak adanya kuota per user. Selesai bila isi bagian itu sesuai dengan tes 5.2 dan 5.3.

## 6. UI `agent/web`

- [x] 6.1 Buat scaffold `agent/web` (React 19, Vite, TypeScript, Vitest) dengan versi sama seperti `sharedservice-oauth/web`, dev server di 5182 yang meneruskan `/auth`, `/chat`, dan `/mcp` ke 8083, serta `api.ts` dan `session.ts` disalin dari `sharedservice-oauth/web`. Selesai bila `heavy npm run typecheck` dan `heavy npm test` di `agent/web` hijau.
- [x] 6.2 Buat `chat.ts` sebagai klien API percakapan dan pembaca SSE dari `fetch` (POST, header CSRF). Selesai bila tes Vitest hijau: parsing event terpecah antar-chunk, event `approval`, dan galat 409/503.
- [x] 6.3 Buat UI: tombol masuk/keluar, pemilih konteks, daftar percakapan (buat, ganti judul, hapus), panel pesan dengan kartu tool, kartu persetujuan Setujui/Tolak, dan kotak input yang nonaktif selama giliran. Semua teks dirender sebagai teks biasa. Selesai bila tes komponen hijau: daftar milik user, alur persetujuan memanggil endpoint approval, dan teks `<img src=x onerror=...>` tampil sebagai teks tanpa elemen `img`.
- [x] 6.4 Tulis `agent/web/README.md` dan `README.en.md` berisi perintah dev, build, dan aturan "UI tidak memegang token". Tambahkan `agent/web` ke tabel dan perintah di `AGENTS.md` dan README akar. Selesai bila `heavy npm run build` menghasilkan `web/dist` dan backend menyajikannya di `/`.

## 7. Image, setup VPS, dan uji integrasi

- [x] 7.1 Jadikan `Dockerfile` di `agent/` multi-stage: Node membangun `web/dist`, Go membangun binary, dan image distroless menjalankan binary dengan `WEB_DIR` ke UI tertanam. Selesai bila image ter-build, lalu `GET /` menyajikan UI, `/healthz` 200, dan `/mcp` tanpa token 401 di container lokal.
- [x] 7.2 Tambahkan fase `agenbff` di `~/.config/nice-vps-register.sh`: perbarui client `agen-backend` dengan redirect `https://agent.unfik.my.id/auth/callback`, post-logout `https://agent.unfik.my.id/`, dan grant `authorization_code` + `refresh_token`. Jalankan dengan token admin `~/.config/nice-vps-admin.json`. Selesai bila respons PUT menampilkan URI dan grant baru, tanpa rahasia tercetak.
- [x] 7.3 Siapkan VPS:
  - buat database dan user `agen` di PostgreSQL VPS;
  - tambahkan `DATABASE_URL`, `LLM_PROVIDER`, `LLM_MODEL`, `LLM_BASE_URL`, `PUBLIC_URL`, `POST_LOGOUT_URL`, dan `WEB_DIR` ke service agen di `/opt/nice/compose.examples.yml` (cadangkan dulu);
  - tulis `LLM_API_KEY` ke `/opt/nice/.env` lewat stdin.

  Tanyakan provider dan model ke user sebelum langkah ini. Selesai bila `docker compose config` valid dan tidak ada rahasia di berkas compose.
- [x] 7.4 Build image dari `agent/`, kirim ke VPS, naikkan `AGEN_IMAGE_TAG`, lalu jalankan `~/.config/nice-vps-agen-smoke.sh` (path smoke disesuaikan ke `agent/backend/scripts/smoke.sh`). Selesai bila semua langkah smoke MCP dan uji persetujuan MCP `ok`.
- [x] 7.5 Jalankan checklist dashboard di browser terhadap `https://agent.unfik.my.id` (masuk sebagai user demo):
  - minta daftar perangkat `nadi`;
  - buat perangkat;
  - hapus dengan Tolak lalu dengan Setujui (perangkat hilang dari `nadi` hanya setelah Setujui);
  - pindah konteks;
  - restart container lalu masuk lagi (riwayat tetap ada);
  - keluar.

  Panel jaringan juga harus tanpa request ke NICE (selain login dan logout), ke provider LLM, atau ke `nadi`. Catat hasilnya di baris "Diuji terhadap" README `agent/backend` (id/en).

## Workflow follow-up

- Setelah semua tugas tercentang dan tes hijau, archive change ini, commit di branch worktree, merge `feat/add-agent-chat-dashboard` ke `main`, lalu hapus worktree dan branch. Push hanya bila diminta.
- Perbarui memori deploy: tag image agen baru dan path build `agent/`.
