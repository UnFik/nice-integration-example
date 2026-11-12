# Design

## Context

`example/` adalah repo baru yang kosong (hanya scaffold OpenSpec). Motivasi dan cakupan ada di `proposal.md`; perilaku yang mengikat ada di tiga spec change ini. Kendala yang membentuk rancangan:

- Kontrak NICE yang diikuti: `nice/docs/token-verification.md` (verifikasi, feed, operasi sensitif), `nice/docs/api.md` bagian "Token OIDC", "Service api_key dan proxy", "Kunci per Service Account", dan "Akses lintas origin", serta `nice/docs/gateway-integration.md`.
- CORS NICE hanya mencakup `/api/v1/*`, `/oauth/token|keys|userinfo|revoke`, dan `/.well-known/*`. Path proxy `/<slug>/...` tidak tercakup, jadi SPA tidak bisa memanggil service `api_key`. Ini alasan ketiga contoh dibuat lepas.
- Client non-sistem tidak mendapat token `aud=nice`, jadi SPA tidak bisa memakai discovery `service-endpoints`. Endpoint service diatur lewat konfigurasi.
- Refresh token NICE hanya terbit bila scope memuat `offline_access`, dan berotasi pada setiap pemakaian. `pkg/niceclient` meminta token service dengan refresh yang hanya ber-scope `nice:aud:<slug> [nice:prj:<id>]`, dan mengunci satu refresh per subject.
- `pkg/nicetoken` dan `pkg/niceclient` tidak dipublikasikan sebagai modul terpisah. Keduanya bagian dari modul `github.com/nice-platform/nice`.
- Operasi key upstream dipanggil NICE dengan kredensial service di header kredensial. Generate dan rotate harus menjawab 2xx dengan JSON `{"key_id","key"}`. `DELETE` dikirim tanpa body, jadi id key diambil dari path. Revoke yang dijawab 404 dianggap sudah dicabut.

## Goals / Non-Goals

**Goals:**
- Setiap contoh bisa dibaca dan disalin sendiri oleh mitra tanpa membuka dua contoh lainnya.
- Bagian yang khas NICE terlihat jelas di kode: wiring `nicetoken`, kepercayaan pada header proxy, dan token manager per audience. Bagian protokol generik diserahkan ke pustaka.
- Tes otomatis tidak butuh NICE yang berjalan. Hanya skrip smoke yang butuh NICE.

**Non-Goals:**
- Penyimpanan data persisten untuk barang dan pengiriman.
- Multi-tab yang berbagi satu sesi di SPA.
- Build produksi, kontainer, atau TLS.
- Memvalidasi tanda tangan ID token di SPA. Token diterima langsung dari endpoint token lewat kanal yang sama, sesuai OIDC untuk alur kode.

## Decisions

### Struktur repo

```
example/
  README.md, README.en.md        indeks dan peta ketiga contoh
  AGENTS.md, CLAUDE.md           panduan agen (CLAUDE.md berisi @AGENTS.md)
  .gitignore                     .env, data/, node_modules/, dist/
  shared-service-oauth/          modul Go, slug inventaris, :8081
  shared-service-apikey/         paket npm, slug pengiriman, :8082
  public-client-pkce/            paket npm (Vite), :5180
```

Setiap contoh punya `README.md`, `README.en.md`, `.env.example`, dan (untuk kedua service) `scripts/smoke.sh`. Tidak ada kode bersama antarcontoh; duplikasi kecil diterima demi kemandirian.

### shared-service-oauth (Go)

- **Modul.** `go.mod` memakai `require github.com/nice-platform/nice` dengan `replace github.com/nice-platform/nice => ../../nice`. Versi Go sama dengan `nice` (1.27). Alternatifnya, menyalin `nicetoken` ke repo ini, ditolak: salinan akan tertinggal dari kontrak dan vector.
- **Wiring `nicetoken`.** `nicetoken.New` dengan `Issuer`, `Service: "inventaris"`, `JWKSFile: data/jwks.json`, `Tenants: ProvisionOnFirstUse(store.CreateTenant)`, dan `Checker: NewHTTPChecker(issuer, key)` hanya bila key `nice.service` diisi; tanpa key, `Checker` nil sehingga operasi sensitif menjawab 503. `v.Run(ctx)` selalu jalan. `v.StartFeed` hanya bila key diisi, selain itu service mencatat peringatan saat start.
- **Rantai handler.** `CORS -> mux`. `/healthz` dan `/openapi.json` dipasang di luar `v.Middleware`. Rute barang dipasang di dalam `v.Middleware` dengan `v.Require(perm[, Sensitive()])`. CORS berada paling luar supaya preflight dijawab tanpa token.
- **Tabel rute tunggal.** Rute barang didefinisikan sebagai tabel `{method, pattern, permission, sensitive, handler}`. Tabel ini dipakai untuk mendaftarkan rute dan untuk tes kecocokan dengan dokumen.
- **OpenAPI dari swag.** swag v1 (sebagai `tool` di `go.mod`, seperti di `nice`) menghasilkan Swagger 2.0 ke `docs/`. Hasilnya di-commit, di-embed, dan disajikan di `/openapi.json`. Izin ditulis sebagai `// @x-nice-permission "inventaris.items.read"` dan `// @x-nice-sensitive true` per operasi. `x-nice-roles` ditulis sebagai ekstensi info umum `// @x-nice-roles [...]` di `main.go`. Handler `/healthz` dan `/openapi.json` tidak diberi anotasi. Alternatifnya, YAML tulis tangan, ditolak oleh pengguna.
- **Tes kecocokan.** Tes membaca dokumen yang di-embed dan memastikan setiap baris tabel rute punya operasi dengan `x-nice-permission` dan `x-nice-sensitive` yang sama, dan tidak ada operasi lain. Ditambah target `make swag-check` yang membandingkan hasil swag terbaru dengan berkas yang di-commit.
- **Nama izin.** Nama izin mengikuti aturan bawaan katalog (`read`/`create`/`edit`/`delete` dari method), sehingga dokumen tetap konsisten walau `x-nice-permission` dihapus. Role `operator` berisi read, create, dan edit.
- **Penyimpanan.** Map per org di memori, dilindungi mutex. ID barang berupa string acak. Galat aplikasi memakai bentuk yang sama dengan `nicetoken`: `{"error","error_description"}`.
- **Tes unit.** `httptest` dengan kunci RSA uji dan `StaticJWKS`, token ditandatangani di tes, dan `AccessChecker` palsu untuk operasi sensitif. Tes tidak butuh NICE.
- **Perintah.** `Makefile` dengan target `run` (`go run .`), `test`, `swag`, dan `swag-check`. `.env` dibaca oleh kode, tanpa dependensi dotenv.

### shared-service-apikey (Hono)

- **Runtime tanpa build.** Node 22.18+ menjalankan `.ts` langsung lewat type stripping (`node --env-file-if-exists=.env src/index.ts`). `tsc --noEmit` dipakai untuk typecheck. Konsekuensinya: tidak ada `enum` atau parameter property, dan impor memakai ekstensi `.ts`. Alternatifnya, `tsx` atau build ke `dist/`, ditolak karena menambah langkah yang tidak relevan dengan integrasi NICE.
- **Server.** `@hono/node-server` mendengarkan di `127.0.0.1:8082`. Service hanya perlu dijangkau NICE.
- **OpenAPI.** Tiga operasi pengiriman didaftarkan dengan `OpenAPIHono` + `createRoute` (zod untuk validasi body), dan `x-nice-permission` diletakkan di objek route. Operasi key, `/healthz`, dan `/openapi.json` didaftarkan sebagai rute Hono biasa, sehingga tidak masuk dokumen. Bila `createRoute` ternyata tidak meneruskan ekstensi `x-`, handler `/openapi.json` menambahkannya ke dokumen hasil berdasarkan `operationId`. Tes memastikan isi dokumen.
- **Autentikasi.** Middleware membandingkan SHA-256 dari `X-Api-Key` dengan `crypto.timingSafeEqual` terhadap hash kredensial service dan hash mirrored key aktif. Konteks request mencatat jenis kredensial (`service` atau `mirrored`). Operasi key menolak jenis `mirrored` dengan 403 `admin_credential_required`.
- **Mirrored key.** Key berbentuk `pgk_` + 32 byte acak base64url, id `key_` + acak. Berkas `data/keys.json` hanya menyimpan `{key_id, label, sha256, status, create_time}`, tidak pernah key utuh. Penulisan memakai berkas sementara lalu `rename`. Rotate mempertahankan `key_id` dan mengganti rahasianya; NICE menerima jawaban apa pun yang memuat `key_id` dan `key`. Generate menjawab 201, rotate 200, revoke 204, dan id tidak dikenal 404.
- **Tenant.** Map per `X-Nice-Organization` di memori. `created_by` diambil dari `X-Nice-Principal`.
- **Tes.** Vitest memanggil `app.request()` tanpa jaringan. Berkas key ditulis ke direktori sementara per tes.

### public-client-pkce (React)

- **Stack.** Vite + React 19 + TypeScript, tanpa router: `App` memeriksa `location.pathname === "/callback"`. CSS biasa. Dev server di port 5180 dengan `strictPort`.
- **Protokol dengan `oauth4webapi`.** Pustaka ini dipakai untuk discovery, PKCE, penukaran kode (termasuk validasi klaim ID token dan `nonce`), refresh, dan revoke. URL end-session dibentuk dari `end_session_endpoint` di discovery. Untuk issuer `http://` hanya pada host loopback, opsi `allowInsecureRequests` dinyalakan; untuk issuer lain opsi itu tidak pernah dipakai. Alternatifnya, `oidc-client-ts` (model satu token per user, dan pustaka memegang refresh token sendiri, sehingga bentrok dengan rotasi lintas audience) dan `fetch` buatan sendiri (lebih banyak kode protokol yang harus dijaga), keduanya ditolak.
- **Scope dasar di setiap refresh.** NICE mengganti scope grant dengan scope yang diminta saat refresh (`SetCurrentScopes`). Karena itu setiap refresh mengirim scope dasar login (`openid profile email offline_access`) ditambah `nice:aud`/`nice:prj`, dan token client (`aud` = `client_id`) diambil dengan scope dasar saja untuk userinfo. Login dan token manager memakai satu konstanta yang sama, karena scope refresh yang bukan subset grant ditolak `invalid_scope`. Ini berbeda dari `pkg/niceclient`, yang hanya mengirim `nice:aud`, dan sesuai panduan NICE pada change `add-seamless-org-switch`.
- **Token manager.** Kelas TypeScript yang tidak bergantung pada React. Penyimpanan, `fetch`, jam, dan acak disuntikkan agar bisa dites. Isinya:
  - cache `Map<"slug|project", Token>`;
  - satu antrean refresh berbasis rantai promise per sesi, untuk memenuhi requirement "Satu refresh sekaligus";
  - peta permintaan yang sedang berjalan per pasangan, sehingga permintaan bersamaan berbagi hasil;
  - kebijakan pakai ulang: token dipakai selama `now < iat + 10 menit + jitter(0-60 detik)` dan `now < exp - 60 detik`;
  - backoff 5 sampai 60 detik setelah kegagalan sementara;
  - callback `onSessionEnded` untuk `invalid_grant` dan `invalid_client`;
  - `call(key, url, init)` yang mengulang sekali pada 401 `token_stale` atau `expired`.

  Perilakunya meniru `pkg/niceclient` (`acquire.go`) supaya kedua SDK berperilaku sama. Refresh token baru disimpan ke `sessionStorage` sebelum promise selesai.
- **Penyimpanan.** Kunci berawalan `nice-example.` di `sessionStorage`: refresh token, ID token, dan transaksi login (verifier, `state`, `nonce`) yang dihapus setelah callback. Access token hanya di memori.
- **Konfigurasi.** `VITE_NICE_ISSUER`, `VITE_NICE_CLIENT_ID`, `VITE_REDIRECT_URI` (bawaan `<origin>/callback`), `VITE_POST_LOGOUT_REDIRECT_URI` (bawaan `<origin>/`), dan `VITE_SERVICES` (JSON array `{slug, endpoint, path, project?}`). Validasi ditulis sendiri tanpa pustaka skema.
- **Tes.** Vitest (environment `node` untuk token manager, `jsdom` hanya bila tes komponen dibutuhkan): PKCE dan `state`, pakai ulang token, refresh berurutan untuk dua service bersamaan, berbagi hasil untuk pasangan yang sama, ulang sekali pada `token_stale`, sesi berakhir pada `invalid_grant`, dan backoff tanpa mengakhiri sesi.

### Setup NICE manual dan smoke

- README setiap contoh berisi checklist bernomor: perubahan `nice/.env`, langkah console, dan potongan `curl` setaranya. Token admin untuk `curl` diambil dengan `nice/scripts/dev-token.sh`. Checklist `inventaris` mencakup Service Account platform berperan `nice.service` beserta API key-nya (feed dan pemeriksa), serta Service Account tenant untuk smoke. Checklist `pengiriman` mencakup `upstream/key-operations` sebagai langkah opsional.
- `scripts/smoke.sh` memakai `bash`, `curl`, dan `jq`, membaca `.env` dengan `set -a`, dan tidak pernah mencetak rahasia. Setiap kegagalan dipetakan ke nomor langkah checklist, mis. `invalid_scope` mengarah ke entitlement, aktivasi, atau binding.
- README mencatat revisi `nice` yang dipakai saat contoh terakhir diuji.

### Konvensi repo

- `AGENTS.md` memuat: dokumen berbahasa Indonesia ditambah `.en.md` untuk README; komentar kode singkat berbahasa Inggris dan diizinkan di ketiga contoh (berbeda dengan console); tidak ada rahasia di berkas yang di-commit; perintah per contoh; dan rujukan ke dokumen kontrak NICE.
- Akar `CLAUDE.md` workspace (di luar repo mana pun, tidak di-commit) ditambah `example/` sebagai repo ketiga. Alur OpenSpec-nya: worktree `.worktrees/example-<change>` dengan branch `feat/<change>` dari `main`, dan `openspec/` ter-track seperti di `nice`.

## Risks / Trade-offs

- [Kontrak NICE berubah dan contoh tertinggal] → Skrip smoke, revisi `nice` yang dicatat di README, dan contoh Go memakai `nicetoken` langsung dari repo saudara, sehingga perubahan kontrak terlihat sebagai kegagalan build atau tes.
- [`replace ../../nice` hanya berjalan bila `nice` berada di sebelah `example/`] → README menjelaskan susunan direktori yang dibutuhkan. Publikasi modul `nice` di luar cakupan.
- [swag tidak mem-parse `@x-` seperti yang diharapkan] → Diverifikasi di tugas pertama contoh Go. Tes kecocokan akan gagal bila ekstensi hilang.
- [`createRoute` tidak meneruskan `x-nice-permission`] → Pasca-proses dokumen berdasarkan `operationId`, dijaga tes isi dokumen.
- [Tab yang diduplikasi menyalin `sessionStorage`, sehingga dua tab memegang refresh token yang sama. Saat tab kedua memakai refresh token yang sudah dirotasi tab pertama, NICE menganggapnya replay dan mencabut seluruh keluarga token (`GetRefreshGrant` → `RevokeFamily`, audit `token.refresh_reuse`), sehingga **kedua** tab berakhir] → Didokumentasikan sebagai batas contoh. Kedua tab menampilkan "Sesi berakhir".
- [Opsi B (scope dasar di setiap refresh) baru dibuktikan terhadap NICE palsu yang meniru aturan subset dan penggantian scope grant] → Dibuktikan ke NICE asli di tugas 5.3 (reload lalu userinfo).
- [svc-apikey memercayai `X-Nice-*` bila kredensial sah] → Service hanya mendengarkan di `127.0.0.1`, dan README memperingatkan bahwa kredensial service tidak boleh diberikan ke pihak selain NICE.
- [Netguard NICE memblokir `http://localhost` secara bawaan] → Langkah pertama checklist: `NICE_OUTBOUND_ALLOW_HTTP` dan `NICE_OUTBOUND_ALLOW_CIDRS`.
- [`allowInsecureRequests` terbawa ke produksi] → Hanya dinyalakan bila issuer memakai `http` pada host loopback; tes memastikan issuer `http` non-loopback ditolak.

## Migration Plan

Tidak ada migrasi; repo baru. Implementasi dikerjakan di worktree `.worktrees/example-add-integration-examples` (branch `feat/add-integration-examples` dari `main`), dengan direktori change dipindahkan ke worktree agar arsipnya ikut di-commit. Setelah semua tugas selesai dan tes hijau, change diarsip, branch di-merge ke `main`, lalu worktree dan branch dihapus. Push hanya bila diminta.

## Open Questions

- Lisensi repo saat nanti diberi remote dan dipublikasikan ke mitra. Tidak memengaruhi isi contoh.
