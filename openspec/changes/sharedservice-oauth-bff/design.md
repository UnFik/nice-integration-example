# Design

## Context

Lihat `proposal.md` untuk alasan. Yang membentuk pendekatan:

- `shared-service-oauth` sudah memisahkan lapisan (`domain`, `app`, `adapter`, `bootstrap`) dan `internal/arch_test.go` menegakkan aturan impor serta akhiran nama berkas. Rantai `CORS -> mux` di `adapter/rest/routes.go` memasang verifikasi `nicetoken` per rute dari tabel rute.
- `service-client-ui` berisi UI yang bisa dipakai ulang (`ui/Home`, `ItemsSection`, `ItemDialogs`, `ContextPicker`, `Toasts`, `Dialog`, `icons`) dan kode protokol yang tidak lagi dibutuhkan (`oidc.ts`, `token-manager.ts`, `contexts.ts`, `jwt.ts`, `storage.ts`, `recent-contexts.ts`, `ManualContextForm`, `TokensSection`).
- Setelah change prasyarat di `nice`: `niceclient.Client` (`KindBFF`) memegang refresh token per subjek, memindah organisasi lewat `Key.Org`, dan memberikan token login (`SignInToken`). `ExchangeClient` menukar token login ke `nice` untuk `/me/contexts`.
- NICE dev berjalan di `localhost:8080`. Cookie tidak dipisahkan menurut port, jadi cookie console dan contoh lain di `localhost` terlihat oleh semua port.

## Goals / Non-Goals

**Goals:**
- Data barang berscope org dan project dari klaim token, dengan jalur PostgreSQL opsional yang bisa disalin mitra.
- Satu contoh fullstack yang bisa disalin mitra: backend memegang OAuth dan sesi, UI hanya memanggil backend.
- Verifikasi token dan dokumen OpenAPI tidak berubah perilakunya; handler barang hanya berubah bahwa datanya kini berscope org dan project.

**Non-Goals:**
- Sinkronisasi penghapusan org atau project dari NICE ke data service.
- Store refresh token produksi (DB/Redis), skala multi-proses, dan akun lokal.
- Mengakhiri sesi SSO NICE saat keluar.

## Decisions

### 1. Struktur folder dan riwayat

```text
sharedservice-oauth/
  backend/          dari shared-service-oauth (git mv), modul Go baru, replace ../../../nice
    internal/app/session/            use case: mulai login, selesaikan callback, konteks, pindah, keluar
    internal/adapter/memory/session/ penyimpanan sesi di memori
    internal/adapter/postgres/item/  barang di PostgreSQL (dipilih bila DATABASE_URL ada); migrations/ di akar backend
    internal/adapter/rest/auth/      /auth/login, /auth/callback, /auth/session, /auth/contexts, /auth/context, /auth/logout
    internal/adapter/rest/web/       penyaji web/dist bila ada
  web/              dari service-client-ui (git mv), tanpa kode protokol
  README.md, README.en.md, .env.example (di backend/)
```

Folder lama dipindah dengan `git mv` supaya riwayat berkas ikut, lalu disesuaikan. Penamaan berkas baru mengikuti aturan `arch_test.go`.

### 2. Subjek SDK terpisah dari ID sesi

Cookie berisi ID sesi acak 32 byte. Rekaman sesi menyimpan `subject` acak lain untuk `niceclient`, konteks aktif `{org, project}`, identitas (nama, email, `usr_`), token CSRF, serta waktu dibuat dan terakhir dipakai. Memisahkan keduanya membuat nilai cookie tidak pernah menjadi kunci penyimpanan token atau muncul di log SDK.

Alternatif: subjek = `sub` NICE. Ditolak: subjek harus ada sebelum login (`AuthorizeURL`), dan dua browser user yang sama tidak boleh berbagi refresh token yang berotasi.

### 3. Middleware sesi mengisi Bearer, lalu rantai yang ada berjalan

Untuk request ke operasi barang tanpa header `Authorization` tetapi dengan cookie sesi: middleware mengambil `Client.Token(Key{Subject, Org, Audience: "inventaris", Project})` untuk konteks aktif, memasangnya sebagai `Authorization: Bearer`, lalu meneruskan ke rantai `nicetoken` yang sama. Request yang sudah membawa `Authorization` diproses sebagai Bearer langsung dan cookie diabaikan.

Alternatif: handler membaca klaim dari sesi tanpa verifikasi. Ditolak: deny-set, feed, kebijakan tenant, dan operasi sensitif harus berlaku sama untuk semua jalur, dan tes OpenAPI tetap menjaga satu tabel rute.

### 4. CSRF: header token sesi dan pemeriksaan Origin

`GET /auth/session` mengembalikan token CSRF sesi. Request tulis ber-cookie wajib membawa `X-CSRF-Token` yang sama dan `Origin` sama dengan `PUBLIC_URL`. Cookie memakai nama khas (`inventaris_session`), `HttpOnly`, `SameSite=Lax`, `Path=/`, dan `Secure` bila `PUBLIC_URL` berskema `https`.

### 5. Konteks: token login ditukar ke `nice`, pindah lewat refresh

- `GET /auth/contexts`: `SignInToken(subject)`, lalu `ExchangeClient.Token(signIn, Key{Audience: "nice"})`, lalu `GET /api/v1/me/contexts?page_size=50` mengikuti semua halaman.
- `POST /auth/context {org, project}`: `Client.Token` untuk kunci tujuan. Berhasil berarti konteks sesi diganti. `GrantError` `access_denied`/`invalid_scope` dijawab 409 dengan alasan, dan konteks tetap.
- Setelah callback, backend membaca konteks dan memilih project pertama organisasi saat ini bila ada, supaya pengguna langsung melihat barang. Bila tidak ada, UI menampilkan pemilih.

### 6. Identitas lewat userinfo sekali saat login

`Config.BaseScopes` diisi `openid profile email offline_access`. Setelah callback, backend memanggil `/oauth/userinfo` dengan token login, lalu menyimpan nama dan email di sesi. `GET /auth/session` mengembalikan identitas, konteks aktif, token CSRF, dan `perms` dari token konteks aktif (dibaca setelah diverifikasi `nicetoken`), sehingga UI bisa menyesuaikan tombol.

### 7. Penyimpanan

`niceclient.MemoryStore` dan sesi di memori. Restart berarti masuk lagi. Sesi berakhir setelah 8 jam tidak dipakai. Saat berakhir atau keluar, backend memanggil `Disconnect`. README menjelaskan Store produksi (DB/Redis terenkripsi dengan `Lock` lintas proses).

### 8. Satu origin di dev dan build

- Dev: Vite di 5181 meneruskan `/auth` dan `/items` ke 8081. `PUBLIC_URL=http://localhost:5181`, redirect URI `http://localhost:5181/auth/callback`.
- Build atau compose: backend menyajikan `web/dist` di 8081. `PUBLIC_URL=http://localhost:8081`, redirect URI `http://localhost:8081/auth/callback`.

Kedua redirect URI didaftarkan di client NICE. CORS backend tetap ada untuk `public-client-pkce` (5180), dan 5181 dihapus dari bawaan.

### 9. Tes

- Backend: NICE palsu (`httptest`) yang menjawab `/oauth/token` (code, refresh dengan `nice:org`, exchange), `/oauth/revoke`, `/oauth/userinfo`, dan `/api/v1/me/contexts`, serta menandatangani token dengan kunci tes yang sudah ada di `helpers_test.go`.
- Backend (PostgreSQL): tes repository terhadap PostgreSQL tes (dilewati bila `TEST_DATABASE_URL` kosong) untuk isolasi org A/B dan project P/Q, `created_by` dari `sub`, dan tenant pertama serentak dengan `-race`.
- Web: Vitest dengan backend palsu (fetch stub) untuk alur masuk, pemilih, pindah konteks, dan galat.

### 10. Data barang per org dan project

Scope berasal dari token, bukan dari request. Handler membaca `nicetoken.FromContext` dan use case meneruskan kunci scope `{OrgID, ProjectID, Subject, Actor}` ke repository; antarmuka repository tidak punya parameter org atau project selain kunci itu. Token tanpa `prj` dijawab 403 `project_required`.

```sql
CREATE TABLE tenants (org_id text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE items (
  id         uuid PRIMARY KEY,
  org_id     text NOT NULL REFERENCES tenants(org_id),
  project_id text NOT NULL,
  created_by text NOT NULL,
  acted_by   text,
  name       text NOT NULL,
  quantity   integer NOT NULL
);
CREATE INDEX items_scope_idx ON items (org_id, project_id);
```

- Setiap query memakai `WHERE org_id = $1 AND project_id = $2`; `GET/PUT/DELETE` by id juga menyertakan keduanya, sehingga id dari scope lain menjadi 404.
- Provisioning tenant: `INSERT INTO tenants ... ON CONFLICT DO NOTHING` sebelum operasi pertama org itu, aman untuk request serentak.
- `DATABASE_URL` memilih adapter PostgreSQL; kosong berarti adapter memori dengan aturan scope yang sama. Tes dan compose lama tetap jalan tanpa database.
- Tanpa foreign key ke NICE: ID org, project, dan user hanyalah teks. Row-level security (`SET LOCAL app.org_id`) disebut di README sebagai pengaman tambahan, tidak diwajibkan.

Alternatif: scope org saja dan `project_id` sebagai kolom informasi. Ditolak: `prj` sudah menjadi kunci scope di token, dan contoh harus menunjukkan pola yang dipakai mitra.

### 11. Label org dan project dari NICE

Token hanya membawa ID. Pemilih konteks mengambil nama dari `GET /auth/contexts` (`/me/contexts`), dan service hanya menyimpan ID. Label boleh di-cache di UI atau sesi, tetapi dimuat ulang saat pemilih dibuka karena nama bisa berubah di NICE.

## Risks / Trade-offs

- **[Cookie `localhost` dibagi semua port]** → Nama cookie khas. Dokumentasi menyebut produksi memakai domain sendiri.
- **[Store memori hilang saat restart]** → Disengaja untuk contoh. README menjelaskan Store produksi.
- **[Dua browser user yang sama membuat dua keluarga token]** → Diterima. NICE mendukung banyak keluarga per user, dan logout hanya mencabut milik sesi itu.
- **[Pemilihan project otomatis bisa memilih project yang tidak diinginkan]** → Pemilih selalu tampil di bilah atas dan pindah konteks hanya satu klik.
- **[Org atau project yang dihapus di NICE tidak memberi tahu service]** → Data tetap ada sampai service membersihkannya sendiri; README menyebutnya dan tidak menambah sinkronisasi.
- **[Token tanpa `prj` ditolak]** → Saat apply, pastikan token Service Account (`smoke.sh`) dan `agent-backend` membawa `prj`; bila tidak, perbarui spec dan skrip sebelum merge.
- **[Ejaan folder lama dan baru hanya beda satu tanda hubung]** → Tugas akhir mencari kedua ejaan di seluruh repo dan mengharuskan hasil kosong untuk ejaan lama.

## Migration Plan

Folder lama dipindah dan dihapus dalam satu change. Mitra yang menyalin contoh lama tetap punya riwayat git. Setup NICE dev perlu client confidential baru, karena client public lama tidak bisa diubah tipenya.
