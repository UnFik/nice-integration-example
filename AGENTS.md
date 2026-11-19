# Contoh integrasi NICE: panduan untuk agen

Repo ini berisi lima contoh integrasi dengan NICE yang **berdiri sendiri**. Contoh-contoh ini tidak berbagi kode. Duplikasi kecil di antaranya disengaja, supaya mitra bisa menyalin satu contoh tanpa membuka contoh lain.

| Folder | Peran di NICE | Stack | Port |
|---|---|---|---|
| `sharedservice-oauth/` | SharedService `nice_token`, slug `nadi` (CRUD perangkat IoT): `backend/` (service Bearer + backend-for-frontend UI sebagai service client confidential) dan `web/` (UI yang hanya bicara ke backend) | Go + `nice/pkg/niceauth/verify` + `nice/pkg/niceauth/credentials`; Vite + React 19 + TypeScript | 8081 (UI build juga di 8081); 5181 saat dev |
| `shared-service-apikey/` | SharedService `api_key`, slug `pengiriman` | TypeScript + Hono (Node 22.18+) | 8082 |
| `public-client-pkce/` | OAuth client tenant `public` (PKCE) | Vite + React 19 + TypeScript | 5180 |
| `agent-backend/` | MCP server untuk SharedService `nice_token` `agen`; tool memanggil service `nice_token` (`inventaris`) lewat token exchange | Go + SDK MCP Go + `nice/pkg/niceauth/credentials` | 8083 |
| `service-account-client/` | Pemanggil mesin: Service Account tenant yang menukar API key ke NICE (`client_credentials`) lalu memanggil service `nice_token` `nadi` | Go + `nice/pkg/niceauth/credentials` (`KindServiceAccount`) | tanpa port (program baris perintah) |

`agent-backend` dan `service-account-client` memakai `replace github.com/nice-platform/nice => ../../nice` dan `sharedservice-oauth/backend` memakai `=> ../../../nice`, jadi repo `nice` harus berada di sebelah `example/`. `compose.yml` di akar menjalankan `sharedservice-oauth` dan PostgreSQL-nya di samping stack NICE di Docker.

## Perintah

```text
cd sharedservice-oauth/backend
make run | make test               jalankan service dan /auth/* / tes unit (tanpa NICE; PostgreSQL bila TEST_DATABASE_URL diisi)
make swag | make swag-check        bangkitkan docs/swagger.json / pastikan masih sama dengan anotasi
scripts/smoke.sh                   uji terhadap NICE yang sudah diatur (baca .env)

cd sharedservice-oauth/web
npm run dev                        http://localhost:5181, /auth dan /items diteruskan ke 8081
npm test | npm run typecheck | npm run build   (build: web/dist, disajikan backend di 8081)

cd shared-service-apikey
npm run dev | npm start            jalankan .ts langsung lewat type stripping Node, tanpa build
npm test | npm run typecheck       Vitest / tsc --noEmit
scripts/smoke.sh                   uji lewat proxy NICE (baca .env)

cd public-client-pkce
npm run dev                        http://localhost:5180
npm test | npm run typecheck | npm run build

cd agent-backend
make run | make test               jalankan agen / tes unit dengan -race (tanpa NICE)
make smoke                         uji terhadap NICE yang sudah diatur (baca .env dan AGENT_USER_TOKEN)

cd service-account-client
make run | make test               jalankan satu perintah (ARGS="list" | "create ..." | "watch") / tes unit dengan -race (tanpa NICE)
make smoke                         uji terhadap NICE dan nadi yang sudah diatur (baca .env)
```

Versi NICE minimum per contoh ada di `README.md` akar. `sharedservice-oauth` dan `agent-backend` butuh `nice` `00256d3` atau sesudahnya (`rename-sdk-to-niceauth`, SDK Go di `nice/pkg/niceauth`), `service-account-client` butuh `df58337` atau sesudahnya (`harden-niceauth-sdk`: `Client.HTTPClient` terikat ke alamat satu service, alamat wajib `https` kecuali loopback), dan `shared-service-apikey` butuh `40ddb20` atau sesudahnya.

## Aturan

- **Dokumen.** Dokumen ditulis dalam bahasa Indonesia. Setiap `README.md` punya `README.en.md` dengan isi yang setara, dan mengubah salah satunya berarti mengubah keduanya.
- **Kode protokol OAuth.** Di browser, `oidc.ts`, token manager, dan NICE palsu di `src/test/` hanya ada di `public-client-pkce`. `sharedservice-oauth/web` tidak boleh memuat kode protokol, token, atau pustaka OAuth: login, refresh, pindah konteks, dan keluar dikerjakan `sharedservice-oauth/backend` lewat `nice/pkg/niceauth/credentials`. Perbaikan protokol yang juga menyentuh SDK diterapkan lewat `niceauth/credentials` di repo `nice`.
- **Komentar kode.** Komentar boleh di semua contoh: singkat, berbahasa Inggris, dan hanya untuk menjelaskan alasan. Jangan menulis komentar yang sekadar mengulang isi kode. Contoh ini bahan ajar, jadi aturannya berbeda dengan `nice-console` yang melarang komentar.
- **Rahasia.** Jangan menulis rahasia ke berkas yang di-commit. `.env` dan `data/` di-ignore, dan `.env.example` hanya berisi placeholder. Nilai kredensial, API key, dan token tidak boleh muncul di log, respons galat, atau keluaran `smoke.sh`.
- **Tes.** Tes unit tidak boleh butuh NICE yang berjalan. Hanya `scripts/smoke.sh` dan uji manual browser yang menyentuh NICE. Tes PostgreSQL `sharedservice-oauth/backend` dilewati tanpa `TEST_DATABASE_URL`; pakai database sementara milik tes sendiri, bukan database NICE.
- **OpenAPI.** Dokumen ini adalah sumber katalog di NICE:
  - Izin di dokumen harus sama dengan yang ditegakkan kode (`agent-backend/docs/openapi_test.go` dan `sharedservice-oauth/backend/internal/bootstrap/openapi_test.go` di Go, serta `test/openapi.test.ts` di Hono, menjaganya). `agent-backend/docs/openapi.json` ditulis tangan, bukan hasil swag.
  - Operasi key (`/keys*`), `/healthz`, `/openapi.json`, dan `/auth/*` (sesi UI `sharedservice-oauth`) tidak boleh masuk dokumen.
  - Setelah mengubah anotasi swag, jalankan `make swag`.
- **Checklist dan smoke.** Nomor langkah checklist di README dirujuk oleh `scripts/smoke.sh`. Mengubah urutan checklist berarti memperbarui skripnya juga.
- **Kontrak NICE.** Kontrak yang diikuti ada di `nice/docs/token-verification.md`, `nice/docs/api.md` ("Token OIDC", "Service api_key dan proxy", "Kunci per Service Account", "Akses lintas origin", "Service Account: `client_credentials`", "Service client", "Token API NICE terdelegasi", "Pindah organisasi lewat refresh", "Konteks pemanggil", "Token exchange: backend atas nama user"), dan `nice/docs/gateway-integration.md`. Bila kontrak berubah:
  1. Perbarui contoh yang terdampak.
  2. Jalankan ulang smoke dan uji manual.
  3. Perbarui baris "Diuji terhadap nice revisi" di README contoh itu.
- **OpenSpec.** `openspec/` ter-track. Change ada di `openspec/changes/`.
