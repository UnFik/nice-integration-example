# service-client-ui-example Specification

## Purpose
Contoh UI service first-party (`inventaris`) yang login sebagai service client NICE, menampilkan organisasi dan project yang bisa dipakai user dari `/api/v1/me/contexts`, dan berpindah konteks lewat refresh tanpa login ulang, sebagai referensi mitra dan sandbox dev.

## Requirements

### Requirement: Login sebagai service client
The UI SHALL sign in through the authorization code flow with PKCE `S256`, `state`, and `nonce` as a public service client bound to `inventaris`, with the base scopes `openid profile email offline_access` and no organization scope, so NICE chooses the first organization of the user entitled to `inventaris`.

#### Scenario: Login pertama
- **WHEN** a user who belongs to organizations A and B, of which only B is entitled to `inventaris`, signs in
- **THEN** the UI shows B as the current organization

#### Scenario: Tidak ada organisasi yang memakai service
- **WHEN** NICE answers the callback with `access_denied`
- **THEN** the UI explains that none of the user's organizations uses the service and offers to sign in with another account

### Requirement: Konteks dari NICE
The UI SHALL obtain a delegated token for audience `nice` with a refresh that carries the base scopes and `nice:aud:nice`, call `GET /api/v1/me/contexts`, and offer an organization and project chooser built only from the answer, with the projects of each organization and the services available there. It SHALL show an organization without usable projects as such and SHALL NOT offer a context the answer does not list.

#### Scenario: Pemilih konteks
- **WHEN** `/me/contexts` lists organization A with projects P and Q and organization B with project R
- **THEN** the chooser offers exactly A/P, A/Q, and B/R, and marks the current one

#### Scenario: Organisasi tanpa project
- **WHEN** `/me/contexts` lists organization C with no project
- **THEN** C is shown as unavailable with the reason that the user holds no `inventaris` access in any of its projects

### Requirement: Pindah konteks lewat refresh
Choosing a context SHALL run one refresh with the base scopes, `nice:org:<org_id>` when the organization changes, `nice:aud:inventaris`, and `nice:prj:<project_id>`, without leaving the page. A refusal (`access_denied` or `invalid_scope`) SHALL keep the previous context and session and show the reason.

#### Scenario: Pindah organisasi
- **WHEN** the user on A/P chooses B/R
- **THEN** the UI sends one refresh with `nice:org:<B> nice:aud:inventaris nice:prj:<R>`, shows the items of B/R, and the browser does not navigate to NICE

#### Scenario: Akses dicabut di antara
- **WHEN** the user chooses a context whose access was removed after `/me/contexts` was read, and NICE answers `invalid_scope`
- **THEN** the UI stays on the previous context, shows the reason, and reloads the contexts

### Requirement: Token per konteks
The UI SHALL keep tokens in memory per `(organization, service, project)`, reuse them under the same rules as `public-client-pkce` (renewal about 10 minutes after issue, one refresh at a time, one retry on `token_stale`), and keep only the refresh token and the ID token in `sessionStorage`. Every refresh SHALL carry the base scopes.

#### Scenario: Kembali ke konteks sebelumnya
- **WHEN** the user returns to A/P within ten minutes of leaving it
- **THEN** the UI reuses the token of A/P for `inventaris` without asking NICE, after a refresh that moves the session back to A only when the organization changes

### Requirement: Panduan setup dan uji
The example SHALL carry `README.md` and `README.en.md` with the NICE version it needs, the manual setup checklist (public OAuth client in a platform organization project bound to `inventaris`, dependency `inventaris` on `nice` with `nice.me.contexts.read`, two entitled organizations), and a manual browser test checklist with expected results for each requirement above.

#### Scenario: Mitra mengikuti README
- **WHEN** a partner follows the setup and test checklists
- **THEN** each step names the expected result, including the refresh request seen in the browser's network panel when switching organization
