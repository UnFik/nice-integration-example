# Contoh integrasi NICE: panduan untuk agen

Repo ini berisi tiga contoh integrasi dengan NICE yang **berdiri sendiri**. Contoh-contoh ini tidak berbagi kode. Duplikasi kecil di antaranya disengaja, supaya mitra bisa menyalin satu contoh tanpa membuka contoh lain.

| Folder | Peran di NICE | Stack | Port |
|---|---|---|---|
| `shared-service-oauth/` | SharedService `nice_token`, slug `inventaris` | Go + `nice/pkg/nicetoken` | 8081 |
| `shared-service-apikey/` | SharedService `api_key`, slug `pengiriman` | TypeScript + Hono (Node 22.18+) | 8082 |
| `public-client-pkce/` | OAuth client tenant `public` (PKCE) | Vite + React 19 + TypeScript | 5180 |

`shared-service-oauth` memakai `replace github.com/nice-platform/nice => ../../nice`, jadi repo `nice` harus berada di sebelah `example/`.

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
```

## Aturan

- **Dokumen.** Dokumen ditulis dalam bahasa Indonesia. Setiap `README.md` punya `README.en.md` dengan isi yang setara, dan mengubah salah satunya berarti mengubah keduanya.
- **Komentar kode.** Komentar boleh di ketiga contoh: singkat, berbahasa Inggris, dan hanya untuk menjelaskan alasan. Jangan menulis komentar yang sekadar mengulang isi kode. Contoh ini bahan ajar, jadi aturannya berbeda dengan `nice-console` yang melarang komentar.
- **Rahasia.** Jangan menulis rahasia ke berkas yang di-commit. `.env` dan `data/` di-ignore, dan `.env.example` hanya berisi placeholder. Nilai kredensial, API key, dan token tidak boleh muncul di log, respons galat, atau keluaran `smoke.sh`.
- **Tes.** Tes unit tidak boleh butuh NICE yang berjalan. Hanya `scripts/smoke.sh` dan uji manual SPA yang menyentuh NICE.
- **OpenAPI.** Dokumen ini adalah sumber katalog di NICE:
  - Izin di dokumen harus sama dengan yang ditegakkan kode (`openapi_test.go` di Go dan `test/openapi.test.ts` di Hono menjaganya).
  - Operasi key (`/keys*`), `/healthz`, dan `/openapi.json` tidak boleh masuk dokumen.
  - Setelah mengubah anotasi swag, jalankan `make swag`.
- **Checklist dan smoke.** Nomor langkah checklist di README dirujuk oleh `scripts/smoke.sh`. Mengubah urutan checklist berarti memperbarui skripnya juga.
- **Kontrak NICE.** Kontrak yang diikuti ada di `nice/docs/token-verification.md`, `nice/docs/api.md` ("Token OIDC", "Service api_key dan proxy", "Kunci per Service Account", "Akses lintas origin"), dan `nice/docs/gateway-integration.md`. Bila kontrak berubah:
  1. Perbarui contoh yang terdampak.
  2. Jalankan ulang smoke dan uji manual.
  3. Perbarui baris "Diuji terhadap nice revisi" di README contoh itu.
- **OpenSpec.** `openspec/` ter-track. Change ada di `openspec/changes/`.
