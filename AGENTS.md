# Contoh integrasi NICE: panduan untuk agen

Repo ini berisi lima contoh integrasi dengan NICE yang **berdiri sendiri**. Contoh-contoh ini tidak berbagi kode. Duplikasi kecil di antaranya disengaja, supaya mitra bisa menyalin satu contoh tanpa membuka contoh lain.

| Folder | Peran di NICE | Stack | Port |
|---|---|---|---|
| `shared-service-oauth/` | SharedService `nice_token`, slug `inventaris` | Go + `nice/pkg/nicetoken` | 8081 |
| `shared-service-apikey/` | SharedService `api_key`, slug `pengiriman` | TypeScript + Hono (Node 22.18+) | 8082 |
| `public-client-pkce/` | OAuth client tenant `public` (PKCE) | Vite + React 19 + TypeScript | 5180 |
| `service-client-ui/` | Service client `public` (PKCE) terikat ke `inventaris` | Vite + React 19 + TypeScript | 5181 |
| `agent-backend/` | MCP server untuk SharedService `nice_token` `agen`; tool memanggil service lain lewat token exchange | Go + SDK MCP Go + `nice/pkg/niceclient` | 8083 |

`shared-service-oauth` dan `agent-backend` memakai `replace github.com/nice-platform/nice => ../../nice`, jadi repo `nice` harus berada di sebelah `example/`.

## Perintah

```text
cd shared-service-oauth
make run | make test               jalankan service / tes unit (tanpa NICE)
make swag | make swag-check        bangkitkan docs/swagger.json / pastikan masih sama dengan anotasi
scripts/smoke.sh                   uji terhadap NICE yang sudah diatur (baca .env)

cd shared-service-apikey
npm run dev | npm start            jalankan .ts langsung lewat type stripping Node, tanpa build
npm test | npm run typecheck       Vitest / tsc --noEmit
scripts/smoke.sh                   uji lewat proxy NICE (baca .env)

cd public-client-pkce
npm run dev                        http://localhost:5180
npm test | npm run typecheck | npm run build

cd service-client-ui
npm run dev                        http://localhost:5181
npm test | npm run typecheck | npm run build

cd agent-backend
make run | make test               jalankan agen / tes unit dengan -race (tanpa NICE)
make smoke                         uji terhadap NICE yang sudah diatur (baca .env dan AGENT_USER_TOKEN)
```

Versi NICE minimum per contoh ada di `README.md` akar. `service-client-ui` butuh `nice` `8160ac4` atau sesudahnya, dan `agent-backend` butuh `6cc9986` atau sesudahnya.

## Aturan

- **Dokumen.** Dokumen ditulis dalam bahasa Indonesia. Setiap `README.md` punya `README.en.md` dengan isi yang setara, dan mengubah salah satunya berarti mengubah keduanya.
- **Kode protokol SPA diduplikasi.** `oidc.ts`, token manager, dan NICE palsu di `src/test/` ada di `public-client-pkce` dan `service-client-ui`. Perbaikan protokol (scope dasar, rotasi, galat token) diterapkan ke keduanya.
- **Komentar kode.** Komentar boleh di semua contoh: singkat, berbahasa Inggris, dan hanya untuk menjelaskan alasan. Jangan menulis komentar yang sekadar mengulang isi kode. Contoh ini bahan ajar, jadi aturannya berbeda dengan `nice-console` yang melarang komentar.
- **Rahasia.** Jangan menulis rahasia ke berkas yang di-commit. `.env` dan `data/` di-ignore, dan `.env.example` hanya berisi placeholder. Nilai kredensial, API key, dan token tidak boleh muncul di log, respons galat, atau keluaran `smoke.sh`.
- **Tes.** Tes unit tidak boleh butuh NICE yang berjalan. Hanya `scripts/smoke.sh` dan uji manual SPA yang menyentuh NICE.
- **OpenAPI.** Dokumen ini adalah sumber katalog di NICE:
  - Izin di dokumen harus sama dengan yang ditegakkan kode (`agent-backend/docs/openapi_test.go` dan `shared-service-oauth/internal/bootstrap/openapi_test.go` di Go, serta `test/openapi.test.ts` di Hono, menjaganya). `agent-backend/docs/openapi.json` ditulis tangan, bukan hasil swag.
  - Operasi key (`/keys*`), `/healthz`, dan `/openapi.json` tidak boleh masuk dokumen.
  - Setelah mengubah anotasi swag, jalankan `make swag`.
- **Checklist dan smoke.** Nomor langkah checklist di README dirujuk oleh `scripts/smoke.sh`. Mengubah urutan checklist berarti memperbarui skripnya juga.
- **Kontrak NICE.** Kontrak yang diikuti ada di `nice/docs/token-verification.md`, `nice/docs/api.md` ("Token OIDC", "Service api_key dan proxy", "Kunci per Service Account", "Akses lintas origin", "Service client", "Token API NICE terdelegasi", "Pindah organisasi lewat refresh", "Konteks pemanggil", "Token exchange: backend atas nama user"), dan `nice/docs/gateway-integration.md`. Bila kontrak berubah:
  1. Perbarui contoh yang terdampak.
  2. Jalankan ulang smoke dan uji manual.
  3. Perbarui baris "Diuji terhadap nice revisi" di README contoh itu.
- **OpenSpec.** `openspec/` ter-track. Change ada di `openspec/changes/`.
