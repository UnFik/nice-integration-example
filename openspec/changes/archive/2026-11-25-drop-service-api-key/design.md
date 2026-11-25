## Context

SDK pasangan change NICE `service-calls-without-service-account` (branch `feat/service-calls-without-service-account` di `nice-auth-sdk`, belum di-tag) mengubah API `verify`:

- `FeedConfig` tidak punya `APIKey`; feed dibaca tanpa `Authorization`, dan setiap perubahan menyebut `key` dan `org_key` (`verify.FeedKey`).
- `NewHTTPChecker(baseURL)` dan `NewHTTPResolver(baseURL)` tanpa key. Keduanya meneruskan token dari `Claims` hasil `Verify` sebagai `Authorization: Bearer`; body pemeriksaan `{"permission": ...}`, body penyelesaian `{}`. `Claims` buatan tangan gagal tertutup.

## Goals / Non-Goals

**Goals:** contoh `nadi` dan `agen` tidak memegang kredensial NICE selain OAuth/service client masing-masing; fitur feed, pemeriksaan online, dan `perms_ref` selalu menyala; tes tetap tanpa NICE nyata.

**Non-Goals:** menaikkan `go.mod` ke tag SDK baru (tag belum ada; tugas terakhir yang dibiarkan terbuka); menjalankan smoke dan uji manual terhadap NICE nyata; mengubah `service-account-client` (API key-nya milik Service Account-nya sendiri).

## Decisions

- **Checker dan resolver selalu dipasang.** Tidak ada lagi saklar konfigurasi, jadi `VerifierConfig` mengembalikan keduanya tanpa syarat dan `StartFeed` selalu membaca feed. Alternatif membiarkan saklar opsional ditolak: tidak ada rahasia yang perlu disediakan, dan fail closed (`503`) tetap berlaku bila NICE tidak terjangkau.
- **Issuer tes `nadi` menjadi `https://nice.test`.** `verify.New` memvalidasi alamat checker dan resolver; `http://nice.test` bukan loopback sehingga ditolak. Dengan `https` validasi lulus tanpa jaringan, dan `newTestEnv` mengganti checker dengan yang diberikan tes (nil berarti tanpa checker, untuk tes fail closed SDK).
- **NICE palsu membaca payload token yang diteruskan.** Penyelesaian dicocokkan dengan `perms_ref` di token (bukan body), dan pemeriksaan/penyelesaian tanpa Bearer dari issuer palsu dijawab `401`, meniru otorisasi NICE. Tes baru memastikan `401` dari NICE menjadi `503` di service.
- **Tes `.env.example` menolak `NICE_SERVICE_API_KEY`** di kedua contoh, supaya variabel itu tidak kembali lewat salin-tempel.
- **Batas long-poll per alamat sumber.** Catatan replika di README tidak lagi menyuruh membuat satu Service Account per 4 replika; NICE kini membatasi long-poll per IP sumber, jadi replika di belakang satu NAT berbagi batas itu.

## Risks / Trade-offs

- [`go.mod` masih `v0.2.2`, kode tidak terkompilasi tanpa `go.work`] → tugas terakhir menaikkan SDK setelah di-tag dan menjalankan tes dengan `GOWORK=off`. Branch tidak di-merge sebelum itu.
- [Contoh tidak jalan terhadap NICE lama] → README dan tabel versi menyebut NICE dengan change `service-calls-without-service-account` sebagai minimum.
- [Belum diuji terhadap NICE nyata] → baris "Diuji terhadap" menyatakan hanya tes unit; smoke dijalankan ulang setelah NICE dan SDK dirilis.
