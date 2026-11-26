# agent/web

[Bahasa Indonesia](README.md)

React UI for the chat dashboard of the `agen` agent. The UI talks only to the backend in [`../backend`](../backend/README.en.md) on the same origin (`/auth/*` and `/chat/*`, with the session cookie and the `X-CSRF-Token` header).

**The UI holds no tokens and never calls NICE, the LLM provider, or any service** (such as `nadi`). It loads no OAuth library either. Sign-in, refresh, switching organization and project, the LLM loop, and tool calls are run by the backend. All text (model answers, tool results, titles) is rendered as plain text, with no `dangerouslySetInnerHTML` and no Markdown renderer. The NICE setup guide and the test checklist are in [`../backend/README.en.md`](../backend/README.en.md).

## Contents

| File | Contents |
| --- | --- |
| `src/api.ts` | Client for `/auth/session`, `/auth/contexts`, `/auth/context`, and `/auth/logout`, plus `send` (JSON) and `open` (a POST answered with SSE): relative URLs, `credentials: "same-origin"`, and `X-CSRF-Token` on every write request. `{"error","error_description"}` and `{"error","message"}` errors become `ApiError` |
| `src/chat.ts` | Client for `/chat/conversations*` and a server-sent events reader over `fetch` (`ReadableStream` + `TextDecoder`), because `EventSource` cannot POST. Frames may be split across chunks or arrive several at once |
| `src/session.ts` | Page state: session, context, conversation list, history, the running turn, approvals, sign-out, and errors (`turn_in_progress`, `chat_not_configured`, `model_unavailable`, 401 `session_ended`, 409 `context_refused`). After a turn ends or the stream is cut, the history is reloaded from `GET /chat/conversations/{id}` |
| `src/ui/`, `src/App.tsx` | Sign-in screen, top bar with the context picker and the account menu (Ganti akun, Keluar), conversation list (create, rename, delete), message panel with tool cards, the Setujui/Tolak (approve/decline) approval card, and the input box |
| `src/test/fake-backend.ts` | Fake `/auth/*` and `/chat/*` backend (SSE included) for the tests, with no backend, NICE, or LLM |

The Masuk (sign in) button is a page navigation to `/auth/login` (the backend redirects to NICE), not a `fetch`. The approval card shows the message the backend built from the service catalog, not model text.

## Running

```bash
npm install
npm run dev        # http://localhost:5182; /auth, /chat, /mcp, /.well-known are forwarded to the backend at 127.0.0.1:8083
npm test           # Vitest with a fake backend, no backend or NICE needed
npm run typecheck
npm run build      # web/dist, served by the backend at / when WEB_DIR points to it
```

In development, run the backend with `PUBLIC_URL=http://localhost:5182`. The Vite proxy keeps the `Origin` header (`changeOrigin: false`), so the backend sees `http://localhost:5182`, and the CSRF check compares it with `PUBLIC_URL`. The `agen-backend` client in NICE also needs the redirect `http://localhost:5182/auth/callback`.

After a build, the backend serves `web/dist` at `/` from `WEB_DIR` (`../web/dist` in the backend `.env.example`), on one origin with `/auth`, `/chat`, and `/mcp`, so port 5182 is no longer needed.
