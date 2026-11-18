# Proposal

## Why

Backend `sharedservice-oauth` mengambil token konteks aktif dari cache `niceclient` lalu memverifikasinya dengan rantai `nicetoken` miliknya sendiri. Setelah feed perubahan akses menandai token itu basi, misalnya karena role user diubah, rantai itu menolaknya dengan `401 token_stale`. Masalahnya, cache `niceclient` tetap menyerahkan token yang sama sampai pembaruan latar belakang berikutnya, paling lama sekitar sepuluh menit. Selama itu UI hanya menerima 401 dan `/auth/session` menampilkan `token_invalid`, padahal NICE akan menerbitkan token baru bila diminta. Hal yang sama akan menimpa operator act-as begitu NICE menandai perubahan haknya.

## What Changes

- Sebelum menyerahkan token konteks aktif ke rantai verifikasi atau ke `/auth/session`, backend memeriksa apakah token itu sudah basi menurut feed.
- Token yang basi dibuang dari cache `niceclient` (`Invalidate`), lalu backend meminta token baru dari NICE **sekali**. Token baru itu yang dipakai.
- Bila token baru pun masih basi, backend tidak mengulang lagi: request dijawab rantai verifikasi seperti biasa (`401 token_stale`). Penolakan NICE saat meminta token baru tetap dijawab seperti sekarang (`409 context_refused`, `401 session_ended`, `503`).
- Request Bearer langsung tidak berubah: pemanggilnya sendiri yang memperbarui token.

## Capabilities

### New Capabilities

(tidak ada)

### Modified Capabilities

- `service-bff-example`: requirement "Validasi token pada setiap request ber-cookie" mendapat aturan pembaruan sekali untuk token konteks yang basi.

## Impact

- `sharedservice-oauth/backend`:
  - use case sesi (`ContextToken`);
  - port `NICE` (membuang token dari cache);
  - gateway `niceclient` (`Client.Invalidate`);
  - pemeriksa kebasian dari `nicetoken.Verifier` yang dipasang di bootstrap;
  - tes use case sesi dan tes bootstrap untuk jalur ber-cookie.
- `sharedservice-oauth/README.md` dan `README.en.md`: bagian request barang dari UI menyebut pembaruan sekali.
- Tidak ada perubahan di `web/`, NICE, atau `nice/pkg/*`.
