# Spec Delta

## MODIFIED Requirements

### Requirement: Validasi token pada setiap request ber-cookie
Untuk request ber-cookie sesi ke operasi barang, backend SHALL mengisi token konteks aktif sebagai `Authorization: Bearer` dan melewatkannya pada verifikasi, izin, dan pemisahan tenant yang sama dengan request Bearer langsung. Tenant SHALL berasal dari `nice_org_id` token, bukan dari cookie, header, parameter, atau body. Bila token konteks aktif sudah basi menurut feed perubahan akses, backend SHALL membuangnya dari cache dan meminta token baru dari NICE tepat sekali sebelum meneruskan request, juga untuk `perms` di `GET /auth/session`. Bila token baru masih basi, backend SHALL NOT meminta lagi dan request dijawab `401 token_stale`.

#### Scenario: Izin dari token
- **WHEN** pengguna yang token konteksnya hanya memuat `inventaris.items.read` membuat barang
- **THEN** backend menjawab 403 `permission_denied` dan tidak membuat barang

#### Scenario: Konteks menentukan tenant
- **WHEN** pengguna pindah dari A/P ke B/R lalu memanggil `GET /items`
- **THEN** daftar memuat barang tenant B dan tidak memuat barang tenant A

#### Scenario: Token konteks basi diperbarui
- **WHEN** feed menandai perubahan akses pengguna di organisasi A sesudah token konteks A/P di cache terbit, lalu pengguna memanggil `GET /items`
- **THEN** backend meminta token baru dari NICE sekali, request diproses dengan token baru, dan `perms` di `GET /auth/session` berasal dari token baru

#### Scenario: Token baru tetap basi
- **WHEN** token yang diterbitkan NICE untuk permintaan ulang itu juga basi
- **THEN** backend tidak meminta token ketiga dan menjawab `401 token_stale`

#### Scenario: NICE menolak token baru
- **WHEN** NICE menolak permintaan token baru dengan `access_denied` karena pengguna tidak lagi boleh memakai konteks itu
- **THEN** backend menjawab `409 context_refused` dan konteks sesi tidak berubah

#### Scenario: Request Bearer langsung
- **WHEN** pemanggil mengirim `Authorization: Bearer` dengan token basi tanpa cookie sesi
- **THEN** backend menjawab `401 token_stale` tanpa meminta token apa pun ke NICE
