# Proposal

## Why

NICE menambah pola baru untuk UI milik service first-party (change `nice`: `add-service-clients`, `add-delegated-nice-api`, `add-seamless-org-switch`, dan change console `service-clients-console`):
- UI didaftarkan sebagai **service client** yang terikat ke satu service;
- UI mengetahui organisasi dan project yang bisa dipakai user lewat `GET /api/v1/me/contexts`, dengan token `aud=nice` terdelegasi;
- UI berpindah organisasi dan project lewat refresh (`nice:org`) tanpa redirect.

Contoh `public-client-pkce` hanya menunjukkan client tenant yang terikat ke satu organisasi dan satu project, jadi mitra yang membangun UI untuk service multi-organisasi belum punya rujukan. Pola ini juga paling mudah salah: scope dasar di setiap refresh, satu refresh token lintas organisasi, dan pemilih konteks yang tidak boleh menebak.

## What Changes

- **Contoh baru `service-client-ui/`** (Vite + React 19 + TypeScript + `oauth4webapi`, port 5181): UI untuk service `inventaris` yang login sebagai service client `public`.
  - Scope login dan refresh mengikuti resep `token-reach` (scope dasar `openid profile email offline_access`).
  - Pemilih organisasi dan project diisi dari `GET /api/v1/me/contexts`, dengan token `aud=nice` terdelegasi yang diambil lewat refresh `nice:aud:nice`.
  - Memilih konteks lain menjalankan satu refresh dengan `nice:org:<org> nice:aud:inventaris nice:prj:<project>`, tanpa redirect.
  - Daftar barang di konteks terpilih, dari `inventaris`.
  - Token manager dengan kunci `(org, service, project)`, antrean refresh tunggal, dan penanganan `access_denied`/`invalid_scope` tanpa merusak sesi.
- **Penyesuaian contoh lain:**
  - `shared-service-oauth`: `.env.example` dan README menambah origin `http://localhost:5181` ke `CORS_ALLOWED_ORIGINS`;
  - `public-client-pkce`: README menjelaskan kapan memakai client tenant dan kapan service client, dan bahwa NICE yang sudah memuat `add-delegated-nice-api` menurunkan origin CORS dari redirect URI client `public`, sehingga `NICE_CORS_ALLOWED_ORIGINS` tidak wajib lagi;
  - README indeks dan `AGENTS.md`: contoh baru, port, perintah, dan versi NICE minimum per contoh.

Di luar cakupan:
- backend agen dan token exchange (change `add-agent-backend-example`);
- operasi tulis ke API NICE lewat token terdelegasi.

## Capabilities

### New Capabilities
- `service-client-ui-example`: contoh UI service first-party yang login sebagai service client, mengambil konteks dari `/me/contexts`, dan berpindah organisasi serta project lewat refresh.

### Modified Capabilities
(tidak ada; capability contoh lain masih berada di change `add-integration-examples` yang belum diarsip, dan penyesuaiannya hanya README dan konfigurasi bawaan, tanpa perubahan perilaku)

## Impact

- **Folder baru** `example/service-client-ui/`. **Berkas yang disesuaikan:** `shared-service-oauth/.env.example` dan README; `public-client-pkce/README*.md`; `README*.md` dan `AGENTS.md` di akar.
- **Butuh NICE** dengan `add-service-clients`, `add-delegated-nice-api`, dan `add-seamless-org-switch`. Uji terhadap NICE dev baru bisa setelah ketiganya digabung.
- **Setup manual di NICE:**
  - OAuth client `public` di project organisasi platform, diikat ke `inventaris`;
  - dependensi `inventaris` → `nice` dengan `nice.me.contexts.read`;
  - user anggota dua organisasi yang meng-entitle `inventaris`.
