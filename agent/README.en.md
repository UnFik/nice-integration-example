# agent (`agen`)

[Bahasa Indonesia](README.md)

An AI agent example for NICE with the slug `agen`. One process serves two entry points to the same tools: an MCP server for MCP clients that bring their own NICE token, and a chat dashboard for users who sign in through the browser. Both find the `nice_token` services and their operations through NICE discovery, then call them on behalf of the user through token exchange.

## Contents

| Folder | Contents |
|---|---|
| [`backend/`](backend/README.en.md) | The MCP server at `/mcp` (Bearer), the dashboard backend-for-frontend (`/auth/*` and `/chat/*`, session cookie), the LLM loop, the conversation history (memory or PostgreSQL), and the server of `web/dist`. Its README has the setup checklist, the variables, the smoke, and the test checklist |
| [`web/`](web/README.en.md) | React 19 + Vite UI: sign-in, context picker, conversation list, chat, and approval cards. It calls only `/auth/*` and `/chat/*` on the same origin |
| `Dockerfile` | One image: builds the UI, then the Go binary that serves the UI at `/` and MCP at `/mcp` |

```text
 MCP client --Bearer JWT (aud=agen)--> backend /mcp
 browser --session cookie--> backend /auth/*, /chat/*  --LLM--> provider (Anthropic or OpenAI-compatible)
 backend --token exchange--> NICE /oauth/token --exchanged Bearer JWT (aud=nadi, act)--> nadi
```
