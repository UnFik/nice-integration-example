# Design

## Context

Motivasi: lihat `proposal.md`. Rancangan SDK-nya: `nice/openspec/changes/rename-sdk-to-niceauth/design.md`. Fakta di repo ini, sudah diperiksa:

- Pemakaian per contoh: `sharedservice-oauth/backend` memakai `nicetoken` (`New`, `Verifier`, `Config`, `ProvisionOnFirstUse`, `AccessChecker`, `HTTPChecker`, `FeedConfig`, `FromContext`, `Sensitive`, `Claims`, `Error`) dan `niceclient` (`New`, `Config`, `KindBFF`, `MemoryStore`, `ConnectRequest`, `Key`, `Token`, `NewExchange`, galat sesi); `agent-backend` memakai `nicetoken` untuk verifikasi dan `niceclient.ExchangeClient`; `service-account-client` hanya `niceclient` (`New`, `Key`, `Do`, galat).
- Nama tipe dan fungsi SDK tidak berubah, hanya jalur impor dan nama paket. Awalan teks galat dan log menjadi `niceauth:`.
- Tes arsitektur ketiga contoh melarang `domain` dan `app` mengimpor apa pun berawalan `github.com/nice-platform/nice/`. Aturan itu tidak menyebut nama paket, jadi tetap menjaga jalur baru.
- Adapter yang membungkus SDK sudah bernama netral terhadap paket: `sharedservice-oauth/backend/internal/adapter/nice`, `agent-backend/internal/adapter/niceexchange`, `service-account-client/internal/adapter/nadi`.
- Stack Docker lokal menjalankan `nadi` dari image yang ikut meng-compile `pkg/` milik `nice`.

## Goals / Non-Goals

**Goals:**

- Ketiga contoh Go ter-build dan lulus tes terhadap `nice` setelah `rename-sdk-to-niceauth`.
- Contoh memperlihatkan bentuk pemakaian SDK yang sekarang dianjurkan, tanpa mengubah perilaku yang sudah diuji.
- Dokumen tidak lagi menyebut nama paket yang sudah tidak ada.

**Non-Goals:**

- Tidak mengubah perilaku, rute, izin, OpenAPI, atau checklist setup contoh mana pun.
- Tidak mengganti nama folder adapter (`adapter/nice`, `adapter/niceexchange`).
- Tidak memakai `Transport` di `sharedservice-oauth/backend` atau `agent-backend`: keduanya meminta token per user (BFF) atau per token subjek (exchange), dan SDK tidak menyediakan transport untuk mode itu.
- Tidak membereskan sebutan `inventaris` dan `/items` yang tersisa di `agent-backend` dan `public-client-pkce`.

## Decisions

### 1. Satu change untuk ketiga contoh Go

Permintaan awalnya menyebut `sharedservice-oauth`, tetapi ketiga contoh rusak oleh sebab yang sama dan diperbaiki dengan cara yang sama. Memperbaiki satu saja meninggalkan dua contoh yang tidak ter-build di `main`. Tugas dikelompokkan per contoh, `sharedservice-oauth/backend` lebih dulu, sehingga tiap grup bisa ditinjau dan diuji sendiri.

### 2. Nama paket tanpa alias, kecuali bentrok

Impor ditulis apa adanya (`credentials.New`, `verify.New`). Alias `nicecredentials` atau `niceverify` hanya dipakai di berkas yang sudah punya pengenal `credentials` atau `verify` sendiri; konvensi ini sama dengan yang ditetapkan change di `nice`. Tujuannya supaya kode contoh terbaca sama dengan contoh di dokumentasi paket SDK.

### 3. `service-account-client` memakai `HTTPClient`, konfigurasi tetap miliknya

- `adapter/nadi` menerima `*http.Client` biasa, bukan antarmuka `Doer` yang meniru `Client.Do`. Client itu dibuat `bootstrap` dengan `client.HTTPClient(credentials.Key{Audience: "nadi"})`. Adapter tidak lagi mengimpor SDK untuk mengirim request; ia hanya mengimpornya untuk menerjemahkan galat (`GrantError`, `IsTemporary`).
- `platform/config` tetap membaca `NICE_ISSUER`, `NICE_SA_ID`, `NICE_SA_API_KEY`, `NADI_URL`, dan `WATCH_INTERVAL` dari environment dan `.env`. `credentials.NewFromEnv` tidak dipakai di program: ia hanya membaca environment proses, sedangkan contoh ini juga membaca `.env` dan menyuntikkan sumber konfigurasi di tes.
- README menambah bagian pendek "bentuk tersingkat" dengan `NewFromEnv` + `HTTPClient`, untuk mitra yang tidak butuh `.env`.

Alternatif: mengekspor isi `.env` ke environment proses lalu memanggil `NewFromEnv`. Ditolak: mengubah environment proses dari dalam program menyulitkan tes dan menyembunyikan dari mana nilai datang.

Karena galat memperoleh token sekarang datang lewat `http.Client.Do`, ia terbungkus `*url.Error`. Urutan pemeriksaan di penerjemah galat adapter harus tetap `GrantError`, lalu `IsTemporary`, baru `*url.Error` sebagai "service tidak terjangkau"; tes lintas lapisan yang ada (`invalid_client`, `invalid_scope`, NICE mati, `nadi` mati) menjaga urutan itu.

### 4. Dokumen dan versi minimum

Semua sebutan `nice/pkg/niceclient` dan `nice/pkg/nicetoken` di README dan `AGENTS.md` menjadi `nice/pkg/niceauth/credentials` dan `nice/pkg/niceauth/verify`. Kutipan log di README (mis. "niceclient: the session ended") mengikuti awalan baru. Versi NICE minimum ketiga contoh Go menjadi revisi merge `rename-sdk-to-niceauth`; hash-nya diisi saat change itu sudah di-merge. Contoh TypeScript mempertahankan versi minimumnya.

### 5. Uji ulang

Tes unit ketiga contoh dijalankan penuh. Karena perilaku tidak berubah, uji terhadap NICE dibatasi pada yang membuktikan SDK baru bekerja di kabel: image `nadi` dibangun ulang, lalu `scripts/smoke.sh` `sharedservice-oauth/backend` dan `service-account-client` dijalankan. `agent-backend` butuh token user dan setup `agen` yang belum ada di stack lokal; smoke-nya dijalankan bila setup itu tersedia, dan bila tidak, README-nya mencatat bahwa revisi baru hanya diuji dengan tes unit.

## Risks / Trade-offs

- [Change ini tidak bisa dikerjakan sebelum `rename-sdk-to-niceauth` di-merge, dan `main` rusak di antara keduanya] → Kerjakan langsung sesudah merge itu; tugas pertama memeriksa revisi checkout `nice`.
- [Worktree berada di `.worktrees/`, sehingga `replace ../../nice` tidak menemukan `nice`] → Buat symlink sementara `.worktrees/nice -> ../nice` selama pengerjaan dan hapus sesudahnya, seperti pada change `add-service-account-client-example`.
- [Penggantian teks massal mengubah kata yang kebetulan memuat nama lama] → Ganti per berkas dengan `gofmt -r` atau penyuntingan impor, bukan `sed` global atas seluruh repo; `grep` akhir memastikan tidak ada sisa.
- [`go` di `PATH` tidak cocok dengan `GOROOT` di mesin pengembang] → Pakai `make GO=/path/ke/go`, sudah didukung ketiga `Makefile`.
- [Baris "Diuji terhadap" diperbarui tanpa uji nyata] → Hanya contoh yang smoke-nya benar-benar dijalankan yang mendapat revisi baru; yang lain menyebut terang bahwa hanya tes unit yang dijalankan.
