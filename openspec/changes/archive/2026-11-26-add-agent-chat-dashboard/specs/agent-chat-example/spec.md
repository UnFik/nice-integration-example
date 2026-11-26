# Spec Delta

## Purpose

Dashboard chat contoh agen. User masuk ke NICE lewat backend agen, lalu meminta LLM bertindak atas namanya memakai tool discovery dan pemanggilan operasi yang sama dengan server MCP. Operasi sensitif tetap butuh persetujuan user di UI.

## ADDED Requirements

### Requirement: Login dashboard lewat backend agen
The agent backend SHALL sign users in with the authorization code flow, PKCE `S256`, and a `state` bound to the local session, as a confidential OAuth client bound to `agen`. It SHALL keep the refresh token on the server and SHALL NOT send any access, refresh, or ID token to the browser. The client secret SHALL appear only in the backend configuration.

#### Scenario: Login berhasil
- **WHEN** the user presses Masuk on the dashboard and finishes the NICE login
- **THEN** the backend exchanges the code, keeps the refresh token server-side, sets a session cookie, and redirects to the dashboard with no token in the URL, body, or browser storage

#### Scenario: Callback tidak cocok
- **WHEN** the callback carries a `state` that is unknown, used, or from another session
- **THEN** the backend creates no session and keeps no token

### Requirement: Sesi, CSRF, konteks, dan keluar dashboard
The dashboard session SHALL follow the backend-for-frontend rules of `sharedservice-oauth`: an `HttpOnly` cookie with a random ID, `SameSite=Lax`, and `Secure` over HTTPS, refusal of cookie-carrying writes without the session's CSRF header or with a foreign `Origin`, organization and project choice read from NICE, context switch by one refresh, and logout that revokes the refresh token.

#### Scenario: Tulis tanpa header CSRF
- **WHEN** a page on another origin posts to `/chat/conversations` with the session cookie
- **THEN** the backend answers 403 and creates nothing

#### Scenario: Pindah konteks
- **WHEN** the user picks another organization and project the user may use with `agen`
- **THEN** the backend sends one refresh with `nice:org`, `nice:aud:agen`, and `nice:prj` for that context, and later tool calls default to it

#### Scenario: Keluar
- **WHEN** the user presses Keluar
- **THEN** the refresh token is revoked at NICE, the cookie is cleared, and the next `/chat/*` request is answered 401

### Requirement: MCP tetap menerima Bearer
Adding the dashboard SHALL NOT change `/mcp`: it SHALL accept only Bearer tokens as before and SHALL ignore the session cookie. `/chat/*` SHALL accept only the session cookie and SHALL NOT accept a Bearer token.

#### Scenario: Cookie ke MCP
- **WHEN** a browser with a dashboard session calls `/mcp` without `Authorization`
- **THEN** the agent answers 401 with the resource metadata challenge

#### Scenario: Bearer ke chat
- **WHEN** a client calls `/chat/conversations` with a Bearer token and no session cookie
- **THEN** the backend answers 401

### Requirement: Percakapan milik satu user
The backend SHALL keep conversations per user, keyed by the `sub` of the session token, with a title, the messages in order, and the time of the last change. A user SHALL list, read, rename, and delete only their own conversations. Another user's conversation SHALL be answered as not found.

#### Scenario: Daftar percakapan
- **WHEN** the user opens the dashboard
- **THEN** the list holds only that user's conversations, newest change first

#### Scenario: Percakapan user lain
- **WHEN** a user requests a conversation ID that belongs to another user
- **THEN** the backend answers 404 and reveals nothing about it

#### Scenario: Hapus percakapan
- **WHEN** the user deletes a conversation
- **THEN** it and its messages are gone from the store and from the list

### Requirement: Penyimpanan riwayat di PostgreSQL
With `DATABASE_URL` set, the backend SHALL store conversations and messages in PostgreSQL and SHALL apply its migrations on start. Without it, the backend SHALL keep them in memory and lose them on restart. The store SHALL NOT hold any access, refresh, ID, or exchanged token, nor the approval state's secret.

#### Scenario: Restart dengan PostgreSQL
- **WHEN** the backend restarts with the same `DATABASE_URL` and the user signs in again
- **THEN** the user's conversations and messages are still listed

#### Scenario: Tanpa token di riwayat
- **WHEN** a conversation that called `call_operation` is read from the database
- **THEN** no row holds a JWT or a refresh token

### Requirement: Giliran chat memakai tool agen
Sending a message SHALL run one turn: the LLM gets the conversation, a system prompt, and the tools `list_services`, `list_operations`, and `call_operation`, with the same inputs, checks, and results as the MCP tools. They run for the session user with the active context as default. A turn SHALL stop after a fixed number of tool steps.

#### Scenario: Minta daftar perangkat
- **WHEN** a user who may call `nadi` in the active project asks for the list of devices
- **THEN** the turn calls `list_services` or `list_operations` and `call_operation` for `GET /devices` of `nadi`, and the answer is built from the devices `nadi` returned

#### Scenario: Langkah dibatasi
- **WHEN** the LLM keeps asking for tools beyond the step limit
- **THEN** the turn ends with a message saying the limit was reached, and no further tool runs

#### Scenario: Tool gagal
- **WHEN** a tool returns an error such as `service_not_allowed`
- **THEN** the error is given to the LLM as the tool result and shown in the conversation, without any token in it

### Requirement: Provider LLM dapat diganti
The backend SHALL support an Anthropic Messages API provider and an OpenAI-compatible Chat Completions provider, chosen with configuration together with the model, the base URL, and the API key. Changing the provider SHALL NOT change the chat API, the tools, or the stored conversations. The API key SHALL appear only in the backend configuration.

#### Scenario: Provider Anthropic
- **WHEN** the backend starts with the Anthropic provider and a key
- **THEN** turns are answered by the Messages API with the configured model

#### Scenario: Provider OpenAI-compatible
- **WHEN** the backend starts with the OpenAI-compatible provider and a base URL such as a local Ollama
- **THEN** turns are answered by that endpoint, and conversations made with the other provider stay readable

#### Scenario: Konfigurasi LLM tidak ada
- **WHEN** the backend starts with no LLM provider configured
- **THEN** `/mcp` still works, and sending a chat message is answered with an error saying the chat is not configured

### Requirement: Persetujuan operasi sensitif di dashboard
When `call_operation` asks for approval of a sensitive operation, the turn SHALL pause and keep the pending call. The UI SHALL show the approval message built from the catalog, not from LLM text, with Setujui and Tolak. Only an approval from the same user SHALL resume the call, once, before it expires.

#### Scenario: User menyetujui
- **WHEN** the LLM calls the sensitive delete of `nadi` and the user presses Setujui
- **THEN** the backend sends the delete to `nadi` once, gives the result to the LLM, and the turn continues

#### Scenario: User menolak
- **WHEN** the user presses Tolak
- **THEN** nothing is sent to the target, no token for it is exchanged, and the LLM is told the user did not approve

#### Scenario: Persetujuan kedaluwarsa
- **WHEN** the user answers after the approval expired
- **THEN** the operation is not called, and the dashboard shows that approval must be asked again

#### Scenario: Persetujuan dari user lain
- **WHEN** another signed-in user posts an answer for that pending call
- **THEN** the backend answers 404 and the call stays pending

### Requirement: UI dashboard hanya bicara ke backend
The UI SHALL send all requests to the backend origin (`/auth/*`, `/chat/*`) with the cookie and the CSRF header. It SHALL NOT call NICE, the LLM provider, or any service, and SHALL NOT keep tokens or load an OAuth library. It SHALL render LLM and tool text as plain text, not as HTML.

#### Scenario: Tidak ada lalu lintas ke luar
- **WHEN** the user signs in, chats, and approves an operation
- **THEN** the browser network panel shows no request to NICE, the LLM provider, or `nadi`, except the navigation to the NICE login and logout pages

#### Scenario: Teks berisi HTML
- **WHEN** a tool result or LLM answer holds `<script>` or `<img onerror>`
- **THEN** the UI shows it as text and runs nothing

### Requirement: Panduan setup dan uji dashboard
The README of `agent/` (id and en) SHALL describe the dashboard setup: the confidential client redirect and post-logout URIs, `refresh_token` grant, LLM variables, and `DATABASE_URL`. It SHALL carry a browser test checklist with expected results: sign-in, a device list by chat, an approved and a declined delete, logout, and the history kept after restart.

#### Scenario: Mitra mengikuti README
- **WHEN** a partner follows the dashboard checklist against NICE and `nadi`
- **THEN** each step names its expected result, and the deleted device is gone from `nadi` only after Setujui
