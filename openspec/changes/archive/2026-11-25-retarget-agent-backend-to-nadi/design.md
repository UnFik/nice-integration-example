## Context

`nadi` (`sharedservice-oauth/backend`) melayani `GET /devices` (`nadi.devices.read`) dan `POST /devices` (`nadi.devices.create`), menjawab daftar sebagai `{"devices":[...]}` dan mencatat `created_by` serta `acted_by` dari token. Body `POST` ditolak bila memuat field yang tidak dikenal. NICE di `eb17848` tidak lagi punya deklarasi dependensi service: token hasil exchange membawa izin user di project tujuan (`nice/docs/api.md`, "Token exchange: backend atas nama user", "Isi token").

## Goals / Non-Goals

**Goals:** contoh berjalan terhadap `nadi` dan NICE `eb17848`; README jujur tentang batas izin; image container.

**Non-Goals:** jalur service tanpa Service Account (`NICE_SERVICE_API_KEY` dan `verify.NewHTTPResolver` tetap), tool ubah atau hapus perangkat, perubahan pada `nadi`.

## Decisions

- **Nama tool `nadi_list_devices` dan `nadi_create_device`.** Pola `<slug>_<aksi>` dipertahankan supaya tes "tidak ada target selain X" tetap bermakna.
- **Skema input mengikuti `DeviceInput` `nadi`.** `name`, `serial_number`, `status` wajib, `location` opsional dan hanya dikirim bila diisi; `status` dibatasi enum `active`, `inactive`, `maintenance` di skema JSON. Validasi panjang tetap di `nadi` (galat `invalid_argument` diteruskan sebagai hasil tool).
- **Tanpa batas izin di sisi agen.** Karena NICE tidak memotong izin lagi, satu-satunya batas yang ditegakkan contoh adalah daftar tool. README mengatakannya, dan `nadi.editor` (tanpa `nadi.devices.delete`) dianjurkan untuk user uji.
- **Container.** `LISTEN_ADDR` bawaan tetap `127.0.0.1:8083` untuk `make run`; di image diisi `0.0.0.0:8083` lewat `ENV` di Dockerfile. `PUBLIC_URL` harus alamat publik `https`, dan `NADI_URL` harus `https` kecuali loopback karena `DoAs` SDK menolak `http` non-loopback.
- **Versi minimum `nice` `5def119`.** Itu awal riwayat `nice` sekarang (2026-11-17), sudah memuat token exchange dan sudah tanpa operasi dependensi. Hanya `eb17848` yang diuji, dan hanya dengan tes unit.

## Risks / Trade-offs

- Smoke belum dijalankan ulang terhadap NICE hidup; README menyatakannya.
