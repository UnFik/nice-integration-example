# agent/web

[English](README.en.md)

UI React untuk dashboard chat agen `agen`. UI ini hanya bicara ke backend di [`../backend`](../backend/README.md) lewat origin yang sama (`/auth/*` dan `/chat/*`, dengan cookie sesi dan header `X-CSRF-Token`).

**UI tidak memegang token dan tidak memanggil NICE, provider LLM, atau service** (mis. `nadi`). UI juga tidak memuat pustaka OAuth. Login, refresh, pindah organisasi dan project, loop LLM, dan pemanggilan tool dijalankan backend. Semua teks (jawaban model, hasil tool, judul) dirender sebagai teks biasa, tanpa `dangerouslySetInnerHTML` dan tanpa renderer Markdown. Panduan setup NICE dan checklist uji ada di [`../backend/README.md`](../backend/README.md).

## Isi

| Berkas | Isi |
| --- | --- |
| `src/api.ts` | Klien `/auth/session`, `/auth/contexts`, `/auth/context`, dan `/auth/logout`, plus `send` (JSON) dan `open` (POST berbalas SSE): URL relatif, `credentials: "same-origin"`, dan `X-CSRF-Token` pada setiap request tulis. Galat `{"error","error_description"}` dan `{"error","message"}` menjadi `ApiError` |
| `src/chat.ts` | Klien `/chat/conversations*` dan pembaca server-sent events dari `fetch` (`ReadableStream` + `TextDecoder`), karena `EventSource` tidak bisa POST. Frame boleh terpotong antar-chunk atau datang beberapa sekaligus |
| `src/session.ts` | Status halaman: sesi, konteks, daftar percakapan, riwayat, giliran yang berjalan, persetujuan, keluar, dan galat (`turn_in_progress`, `chat_not_configured`, `model_unavailable`, 401 `session_ended`, 409 `context_refused`). Setelah giliran selesai atau terputus, riwayat dimuat ulang dari `GET /chat/conversations/{id}` |
| `src/ui/`, `src/App.tsx` | Halaman masuk, bilah atas dengan pemilih konteks dan menu akun (Ganti akun, Keluar), daftar percakapan (buat, ganti judul, hapus), panel pesan dengan kartu tool, kartu persetujuan Setujui/Tolak, dan kotak input |
| `src/test/fake-backend.ts` | Backend palsu `/auth/*` dan `/chat/*` (termasuk SSE) untuk tes, tanpa backend, NICE, atau LLM |

Tombol Masuk adalah navigasi halaman ke `/auth/login` (backend mengalihkan ke NICE), bukan `fetch`. Kartu persetujuan menampilkan pesan yang disusun backend dari katalog service, bukan teks model.

## Menjalankan

```bash
npm install
npm run dev        # http://localhost:5182; /auth, /chat, /mcp, /.well-known diteruskan ke backend di 127.0.0.1:8083
npm test           # Vitest dengan backend palsu, tanpa backend atau NICE
npm run typecheck
npm run build      # web/dist, disajikan backend di / bila WEB_DIR menunjuk ke sana
```

Saat dev, jalankan backend dengan `PUBLIC_URL=http://localhost:5182`. Proxy Vite tidak mengubah header `Origin` (`changeOrigin: false`), jadi backend melihat `http://localhost:5182`, dan pemeriksaan CSRF membandingkannya dengan `PUBLIC_URL`. Client `agen-backend` di NICE juga butuh redirect `http://localhost:5182/auth/callback`.

Setelah build, backend menyajikan `web/dist` di `/` dari `WEB_DIR` (`../web/dist` di `.env.example` backend), satu origin dengan `/auth`, `/chat`, dan `/mcp`, jadi port 5182 tidak dibutuhkan lagi.
