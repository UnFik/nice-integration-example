# sharedservice-oauth/web

[Bahasa Indonesia](README.md)

React UI for the `nadi` service. The UI talks only to the backend in [`../backend`](../backend) on the same origin: it holds no tokens, never calls NICE, and loads no OAuth library. Sign-in, refresh, switching organization and project, and context discovery are done by the backend. The NICE setup guide and the test checklist are in [`../README.en.md`](../README.en.md).

## Contents

| File | Contents |
| --- | --- |
| `src/api.ts` | Client for `/auth/session`, `/auth/contexts`, `/auth/context`, `/auth/logout`, and `/devices*`: relative URLs, the session cookie (`credentials: "same-origin"`), and `X-CSRF-Token` on every write request |
| `src/session.ts` | Page state: session, active context and `perms`, context choices, context switch, devices, sign-out, and the `login_error`, 401 `session_ended`, 409 `context_refused`, and sign-out 503 errors |
| `src/devices.ts` | Device input validation and error sentences |
| `src/ui/`, `src/App.tsx` | Sign-in screen, top bar with the active context, Context panel, devices table, and add/edit/delete dialogs. Buttons follow `perms` from `GET /auth/session` |

The Masuk (sign in) button is a page navigation to `/auth/login` (the backend redirects to NICE), not a `fetch`.

## Running

```bash
npm install
npm run dev      # http://localhost:5181, /auth and /devices are forwarded to the backend at 127.0.0.1:8081
npm test         # Vitest with a fake backend (src/test/fake-backend.ts), no backend or NICE needed
npm run typecheck
npm run build    # web/dist, served by the backend on 8081
```

The Vite proxy keeps the `Origin` header (`changeOrigin: false`), so the backend sees `http://localhost:5181`. That value must equal the backend `PUBLIC_URL` in dev, because the CSRF check compares them.
