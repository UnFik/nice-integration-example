# agent (`agen`)

[English](README.en.md)

Contoh agen AI untuk NICE dengan slug `agen`. Satu proses melayani dua pintu masuk ke tool yang sama: server MCP untuk client MCP yang membawa token NICE sendiri, dan dashboard chat untuk user yang masuk lewat browser. Kedua pintu menemukan service `nice_token` dan operasinya lewat discovery NICE, lalu memanggilnya atas nama user lewat token exchange.

## Isi

| Folder | Isi |
|---|---|
| [`backend/`](backend/README.md) | Server MCP di `/mcp` (Bearer), backend-for-frontend dashboard (`/auth/*` dan `/chat/*`, cookie sesi), loop LLM, riwayat percakapan (memori atau PostgreSQL), dan penyaji `web/dist`. README-nya memuat checklist setup, variabel, smoke, dan checklist uji |
| [`web/`](web/README.md) | UI React 19 + Vite: masuk, pemilih konteks, daftar percakapan, chat, dan kartu persetujuan. Hanya memanggil `/auth/*` dan `/chat/*` di origin yang sama |
| `Dockerfile` | Satu image: membangun UI, lalu binary Go yang menyajikan UI di `/` dan MCP di `/mcp` |

```text
 MCP client --Bearer JWT (aud=agen)--> backend /mcp
 browser --cookie sesi--> backend /auth/*, /chat/*  --LLM--> provider (Anthropic atau OpenAI-compatible)
 backend --token exchange--> NICE /oauth/token --Bearer JWT hasil exchange (aud=nadi, act)--> nadi
```
