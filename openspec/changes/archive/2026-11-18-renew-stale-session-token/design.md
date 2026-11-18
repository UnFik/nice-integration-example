# Design

## Context

- **Request ber-cookie.** `WithSession` (`internal/adapter/rest/auth/session_auth_middleware.go`) memanggil `Sessions.ContextToken`, lalu memasang hasilnya sebagai `Authorization: Bearer` sebelum rantai `nicetoken.Middleware` → `Require`.
- **`/auth/session`.** `body` (`session_auth_helper.go`) juga memakai `ContextToken`, lalu `Perms`, yang memanggil `Verifier.Verify`.
- **Asal token.** `ContextToken` → `NICE.ServiceToken` → `niceclient.Client.Token(Key{Subject, Org, "inventaris", Project})`. Client menyimpan token per key dan memperbaruinya di latar belakang sekitar sepuluh menit setelah terbit, tanpa tahu isi deny set `nicetoken`.
- **API yang tersedia.** `niceclient.Client.Invalidate(key, Token{Access})` membuang token di cache bila masih sama, sehingga `Token` berikutnya langsung meminta yang baru. `nicetoken.Verifier.Verify` mengembalikan `*nicetoken.Error` dengan `Description` `token_stale` untuk token basi.

## Goals / Non-Goals

**Goals:**
- Token konteks yang basi diganti sebelum dipakai, cukup dengan satu permintaan ke NICE.
- Satu jalur untuk request barang dan `/auth/session`.

**Non-Goals:**
- Mencoba ulang request yang sudah dijawab rantai verifikasi (membuffer dan mengulang handler).
- Mengubah perilaku request Bearer langsung, `niceclient`, atau `nicetoken`.

## Decisions

### D1. Periksa kebasian di use case sebelum token dipakai
`sessionusecase.Service` mendapat port `Stale func(ctx, token) bool`. `ContextToken` mengambil token, lalu bila `Stale` bernilai benar:
1. memanggil `NICE.DropServiceToken(subject, context, token)`;
2. mengambil token sekali lagi;
3. menyerahkan hasilnya apa adanya, sehingga token yang masih basi ditolak rantai verifikasi dengan `401 token_stale`.

Alasan memilih jalur ini:
- `WithSession` dan `body` sama-sama lewat `ContextToken`, jadi keduanya mendapat perbaikan tanpa duplikasi.
- Pemeriksaan bersifat lokal (deny set di memori), jadi tidak ada panggilan jaringan tambahan pada jalur biasa.

Alternatif yang ditolak: membungkus `ResponseWriter` untuk menangkap `401 token_stale` lalu mengulang handler. Cara ini membuffer body, dan mengulang request tulis bisa menjalankan efek dua kali.

### D2. `Stale` dari `Verifier.Verify` di bootstrap
Bootstrap memasang `Stale` sebagai `Verify` yang hasilnya `*nicetoken.Error` dengan `Description == nicetoken.ReasonStale`. Galat lain (kedaluwarsa, tanda tangan) tidak dianggap basi: yang pertama sudah ditangani `niceclient` lewat masa aman, yang kedua bukan urusan pembaruan. `Stale` yang nil (tes, tanpa feed) berarti tidak pernah basi.

### D3. `DropServiceToken` di gateway
`Gateway.DropServiceToken` memanggil `Client.Invalidate(Key{...}, niceclient.Token{Access: token})`. Karena `Invalidate` hanya membuang bila token di cache masih sama, dua request serentak tidak saling membuang token baru milik yang lain.

## Risks / Trade-offs

- **[Verifikasi dua kali per request ber-cookie]** Sekali di `ContextToken` dan sekali di rantai. → Diterima: verifikasi RS256 lokal murah dibanding request, dan JWKS sudah di-cache.
- **[Token yang terbit pada detik yang sama dengan perubahan]** Deny set beresolusi detik, jadi token itu tidak tertangkap (perilaku `nicetoken`). → Tidak berubah oleh change ini.
- **[Beban ke NICE saat banyak token basi bersamaan]** Satu permintaan per key. `niceclient` menyatukan permintaan serentak untuk key yang sama (`flight`).
