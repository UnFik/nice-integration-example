# service-account-client

[English](README.en.md)

Contoh **pemanggil mesin**: integrasi tanpa user (job, daemon, service mitra) yang memegang API key sebuah Service Account, membuktikannya ke NICE, lalu memanggil service `nice_token` `nadi` ([`../sharedservice-oauth/backend`](../sharedservice-oauth/backend/README.md)) dengan token hasilnya. Ditulis dalam Go di atas `nice/pkg/niceclient` dengan `KindServiceAccount`. Empat hal yang diperlihatkan:

- API key **hanya dikirim ke endpoint token NICE**. `nadi` tidak pernah melihat key, hanya `Authorization: Bearer <token>`;
- satu token dipakai ulang selama masih berlaku, dan diperbarui di latar sebelum kedaluwarsa;
- bila `nadi` menjawab 401 `token_stale` atau `expired`, token baru diambil dan request dikirim **satu kali** lagi;
- bila NICE menolak key secara permanen (`invalid_client`), klien berhenti meminta token dan keluar dengan penjelasan.

```text
klien --client_credentials: client_id=sa_..., client_secret=API key, scope=nice:aud:nadi--> NICE /oauth/token
klien --Bearer JWT (aud=nadi, nice_principal_type=service_account)--> nadi /devices
```

Versi NICE minimum: `nice` `c7a5086` atau sesudahnya (sama dengan jalur Bearer `nadi`).

Diuji terhadap nice revisi: `c7a3e33` (2026-11-19), NICE dan `nadi` di Docker: `scripts/smoke.sh`, `create` lalu `list` (`created_by` berisi id Service Account), `create` dengan role `nadi.viewer` (403 `permission_denied`), dan key dicabut saat `watch` berjalan (`watch` berhenti dengan `invalid_client` pada pembacaan berikutnya, feed aktif di `nadi`).

## Struktur folder

Polanya sama dengan `agent-backend` dan `sharedservice-oauth/backend`, yang meniru `nice/internal/modules` (lihat `nice/AGENTS.md`), dipangkas untuk satu resource (`device`) tanpa repository dan facade.

```text
cmd/service-account-client/main.go   tipis: slog, sinyal, memanggil bootstrap.Run, kode keluar
internal/
  domain/                            aturan murni (*_domain.go): slug dan izin nadi, Device dan statusnya,
                                     jenis galat (GrantError, ErrKeyRefused, ErrTemporary, ErrUnreachable, ServiceError)
  app/device/
    usecase/                         satu aksi per berkas: ListDevices, CreateDevice, WatchDevices (*_usecase.go),
                                     Service (service_device_usecase.go), kontrak Gateway dan Waiter (device_contract.go)
    dto/                             CreateInput, Reading, Failure (*_dto.go)
    internal/failure/                galat menjadi Failure: pesan dan langkah checklist (*_helper.go)
  adapter/
    cli/                             tabel perintah (routes.go) dan satu handler per perintah (*_handler.go);
                                     internal/mapper/ mencetak perangkat dan Failure (*_mapper.go)
    nadi/                            port ke nadi lewat niceclient (*_gateway.go)
  platform/config/                   env dan .env
  bootstrap/                         perakitan; tes lintas lapisan dengan NICE dan nadi palsu
  arch_test.go                       tes arsitektur: akhiran nama berkas, satu fungsi per use case, arah impor
scripts/smoke.sh                     uji smoke terhadap NICE yang sudah diatur
```

Alasannya: `main.go` hanya merakit; aturan dan use case tidak mengimpor `net/http` atau `niceclient` (hanya `adapter/nadi` yang mengenal SDK, ditegakkan `internal/arch_test.go`); setiap berkas membawa lapisannya di akhiran nama (`_domain`, `_usecase`, `_contract`, `_dto`, `_handler`, `_gateway`, `_helper`, `_mapper`); satu `*_usecase.go` berisi satu fungsi bernama sama dengan berkasnya.

## Perintah

| Perintah | Izin `nadi` | Yang dikerjakan |
|---|---|---|
| `list` | `nadi.devices.read` | mencetak perangkat project milik Service Account sebagai tabel |
| `create -name N -serial S [-status ST] [-location L]` | `nadi.devices.create` | mendaftarkan satu perangkat dan mencetak id-nya. `-status` bawaannya `active` (`active`, `inactive`, `maintenance`) |
| `watch` | `nadi.devices.read` | mencetak perangkat setiap `WATCH_INTERVAL` sampai dihentikan dengan Ctrl+C |

Hasil ditulis ke stdout, log dan galat ke stderr. Kode keluar `0` bila berhasil (termasuk `watch` yang dihentikan operator), `1` bila gagal, `2` bila perintah atau flag salah.

Tidak ada perintah ubah dan hapus. `DELETE /devices/{id}` operasi sensitif yang diperiksa online di sisi `nadi`; itu tidak menambah pelajaran tentang sisi klien.

### Cara kerja panggilan

`bootstrap` membuat satu `niceclient.Client` (`Kind: KindServiceAccount`, `ClientID` = id Service Account, `ClientSecret` = API key) dan menjalankan `Run` di latar. Setiap panggilan lewat `client.Do(ctx, niceclient.Key{Audience: "nadi"}, req)`:

1. Belum ada token: SDK mengirim `client_credentials` ke `/oauth/token` dan menyimpan tokennya di memori.
2. Token masih berlaku: dipakai ulang, tanpa menghubungi NICE.
3. Sepuluh menit setelah terbit (token berumur 30 menit), `Run` mengambil token baru di latar. Panggilan yang sedang berjalan tetap memakai token lama.
4. `nadi` menjawab 401 `token_stale` atau `expired`: SDK membuang token itu, mengambil yang baru, dan mengirim request sekali lagi. Jawaban kedua dikembalikan apa adanya.
5. NICE menjawab `invalid_client`: SDK berhenti meminta token untuk Service Account itu. Panggilan berikutnya langsung gagal tanpa menghubungi NICE, dan klien keluar.

Project tidak disebut di scope: token Service Account selalu untuk project tempat akun itu dibuat. Tidak ada refresh token; "memperbarui" berarti membuktikan key lagi.

Padanan `curl` satu panggilan `list`, untuk bahasa lain:

```bash
set -a; . ./.env; set +a
TOKEN=$(curl -sS "$NICE_ISSUER/oauth/token" -d grant_type=client_credentials -d scope=nice:aud:nadi \
  --data-urlencode "client_id=$NICE_SA_ID" --data-urlencode client_secret@- <<<"$NICE_SA_API_KEY" | jq -r .access_token)
curl -sS "${NADI_URL:-http://127.0.0.1:8081}/devices" -H "Authorization: Bearer $TOKEN" | jq .
```

### Galat

Setiap kegagalan dicetak ke stderr sebagai `FAILED: <kode> (<deskripsi>): <penjelasan>`, diikuti langkah checklist yang perlu diperiksa. Pesan hanya memuat kode dan deskripsi dari NICE atau `nadi`, tidak pernah key, token, atau body mentah.

| Kode | Asal | Arti | Langkah | `watch` |
|---|---|---|---|---|
| `invalid_client` | NICE | key salah, dicabut, atau kedaluwarsa; Service Account nonaktif; atau `NICE_SA_ID` bukan pemilik key | 4 | berhenti |
| `invalid_scope` | NICE | `nadi` belum di-entitle organisasi atau belum aktif di project Service Account | 2 | berhenti |
| `nice_unavailable` | klien | NICE tidak menjawab, 5xx, atau 429 | | lanjut |
| `service_unavailable` | klien | `nadi` tidak terjangkau | 1 | lanjut |
| `insufficient_scope` (`permission_denied`) | `nadi` 403 | token tidak memuat izin perintah itu | 3 | berhenti |
| `invalid_token` (`token_stale`) | `nadi` 401 | akses berubah dan token baru pun masih ditolak | | berhenti |
| `invalid_token` (`wrong_issuer`, `wrong_audience`) | `nadi` 401 | `nadi` mempercayai NICE lain, atau terdaftar dengan slug lain | 1 | berhenti |
| `invalid_argument` | `nadi` 400 atau klien | field `create` tidak sah; deskripsinya nama field | | |
| jawaban 5xx atau 429 | `nadi` | `nadi` sedang tidak bisa menjawab | | lanjut |

`watch` melanjutkan hanya untuk kegagalan yang bisa pulih tanpa mengubah setup, dan berhenti dengan kode keluar `1` untuk yang lain. Selama NICE mati, token yang belum kedaluwarsa tetap dipakai.

## Prasyarat

- Susunan direktori: `replace github.com/nice-platform/nice => ../../nice` di `go.mod` hanya berjalan bila repo `nice` berada di sebelah `example/`.
- Go 1.27, `curl`, `jq`, dan `openssl` (untuk `nice/scripts/dev-token.sh`).
- NICE dengan versi minimum di atas, bawaannya di `http://localhost:8080`.
- `nadi` berjalan dan sudah diatur sampai langkah 7 [`../sharedservice-oauth/backend/README.md`](../sharedservice-oauth/backend/README.md).

## Konfigurasi

Salin `.env.example` ke `.env`. Klien membaca `.env` di direktori tempat ia dijalankan; variabel di environment menang atas berkas.

| Variabel | Bawaan | Arti |
|---|---|---|
| `NICE_ISSUER` | wajib | issuer NICE, sama persis dengan klaim `iss` token |
| `NICE_SA_ID` | wajib | id Service Account tenant (`sa_...`, langkah 2), dikirim sebagai `client_id` |
| `NICE_SA_API_KEY` | wajib | API key Service Account itu (langkah 4). Hanya dikirim ke endpoint token NICE |
| `NADI_URL` | `http://127.0.0.1:8081` | alamat `sharedservice-oauth/backend` |
| `WATCH_INTERVAL` | `30s` | jeda `watch`, dalam format durasi Go |

Variabel wajib yang kosong menghentikan klien sebelum ada panggilan jaringan, dengan pesan yang menyebut namanya.

## Menjalankan dan tes

```bash
make run                       # list
make run ARGS="create -name 'Sensor Pintu' -serial SN-2026-0002 -location 'Gudang B'"
make run ARGS=watch
make test                      # go test -race; tidak butuh NICE (NICE dan nadi palsu)
make smoke                     # scripts/smoke.sh, butuh NICE yang sudah diatur dengan checklist di bawah
```

Bila `go` di `PATH` tidak cocok dengan `GOROOT`, panggil `make GO=/path/ke/go test`.

Tes di `internal/bootstrap` menjalankan perakitan asli dengan `niceclient.Client` asli di depan NICE palsu (endpoint token `client_credentials`) dan `nadi` palsu, jadi yang dibuktikan adalah perilaku di kabel: berapa kali `/oauth/token` dipanggil dan header apa yang sampai ke service. Jam SDK dipalsukan supaya pembaruan token tidak menunggu sepuluh menit. Setiap tes ditutup dengan pemeriksaan yang sama: key tidak pernah muncul di request ke `nadi`, dan key maupun token tidak pernah muncul di stdout atau stderr.

## Checklist setup NICE

Jalankan dari direktori `example/service-account-client`, dalam satu sesi shell. Langkah console disebut dengan nama halamannya; potongan `curl` adalah padanannya. Password dibaca dari terminal dan tidak pernah ditulis ke berkas.

```bash
export NICE=${NICE:-http://localhost:8080}   # sama dengan NICE_ISSUER di .env; dev-token.sh juga membacanya

read -rp 'Email owner organisasi tenant: ' OWNER_EMAIL; read -rsp 'Password: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD"); unset OWNER_PASSWORD
PROJECT_ID=prj_...   # project tenant tempat nadi aktif
```

Token berlaku 30 menit; ambil ulang bila `curl` dijawab 401.

### 1. `nadi` berjalan dan terdaftar

Ikuti [`../sharedservice-oauth/backend/README.md`](../sharedservice-oauth/backend/README.md) langkah 1 sampai 7: service berjalan di `http://127.0.0.1:8081`, terdaftar sebagai `nadi`, katalognya diimpor, aktif, di-entitle organisasi tenant, dan aktif di `PROJECT_ID`. Langkah 9 contoh itu (`NICE_SERVICE_API_KEY`) tidak wajib, tetapi dibutuhkan untuk mencoba `token_stale`.

### 2. Service Account tenant

Console: project tenant, **Service Account**.

```bash
SA_ID=$(curl -sS "$NICE/api/v1/projects/$PROJECT_ID/service-accounts" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"nadi-client"}' | jq -r .service_account.id)
echo "$SA_ID"
```

Service Account milik project tempat ia dibuat. Token untuk `nadi` hanya terbit bila project itu mengaktifkan `nadi`.

### 3. Binding ke role `nadi`

Console: project tenant, **IAM**.

```bash
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/role-bindings" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_SERVICE_ACCOUNT\",\"principal_id\":\"$SA_ID\",\"role\":\"nadi.editor\"}"
```

`nadi.editor` memuat `nadi.devices.read`, `.create`, dan `.edit`. Key hanya membuktikan identitas akun; izinnya datang dari binding ini. Untuk integrasi yang hanya membaca, pakai `nadi.viewer`.

IAM mengetahui Service Account baru secara asinkron, jadi binding yang dibuat tepat setelah akunnya dibuat bisa dijawab 400 `principal_not_in_org`. Tunggu beberapa detik lalu ulangi.

### 4. API key dan `.env`

Console: project tenant, **Service Account**, akun `nadi-client`, **API key**.

```bash
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/api-keys" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"service_account_id\":\"$SA_ID\",\"name\":\"nadi-client\"}" | jq '{id: .api_key.id, key}'
```

Key hanya tampil sekali. Salin `.env.example` ke `.env`, lalu isi `NICE_SA_ID` dengan `$SA_ID` dan `NICE_SA_API_KEY` dengan key itu. `.env` tidak di-commit.

### 5. Smoke

```bash
scripts/smoke.sh
```

Skrip membaca `.env` saja, tidak pernah mencetak key atau token, dan memeriksa: NICE menjawab, `nadi` menjawab `/healthz`, lalu perintah `list` berhasil. Skrip tidak membuat perangkat, jadi aman dijalankan berulang. Kegagalan dipetakan ke langkah checklist:

| Jawaban | Langkah yang kemungkinan terlewat |
|---|---|
| `.env` tidak ada, `NICE_SA_ID` kosong | 2 |
| `NICE_ISSUER` atau `NICE_SA_API_KEY` kosong | 4 |
| NICE tidak terjangkau | NICE harus berjalan, dan `NICE_ISSUER` harus alamatnya |
| `nadi` tidak terjangkau, `service_unavailable` | 1 |
| `invalid_client` | 4: id Service Account bukan pemilik key, key dicabut atau kedaluwarsa, akun nonaktif |
| `invalid_scope` | 2: project Service Account harus mengaktifkan `nadi` (langkah 1) |
| `permission_denied` | 3 |

## Checklist uji

1. **Buat lalu baca.** Perangkat baru muncul di daftar, dan kolom `CREATED BY`-nya id Service Account (`sa_...`):

   ```bash
   make run ARGS="create -name 'Sensor Pintu' -serial SN-2026-0002"
   make run
   ```

2. **Role hanya baca.** Ganti binding langkah 3 dengan `nadi.viewer` (atau pakai Service Account lain yang hanya memegang `nadi.viewer`), lalu jalankan `create` lagi. Jawabannya `FAILED: insufficient_scope (permission_denied)` dengan langkah 3, dan kode keluar `1`. `list` tetap berhasil.

   Perubahan binding baru terlihat di token berikutnya. Proses `list` dan `create` selalu mengambil token baru, jadi langsung terlihat.

3. **Key dicabut saat `watch` berjalan.** Jalankan `watch`, lalu cabut key-nya dari Console (**Service Account**, **API key**, cabut) atau dengan `curl`:

   ```bash
   WATCH_INTERVAL=10s make run ARGS=watch
   # di terminal lain; KEY_ID adalah .api_key.id dari jawaban langkah 4
   curl -sS -X DELETE "$NICE/api/v1/projects/$PROJECT_ID/api-keys/$KEY_ID?service_account_id=$SA_ID" \
     -H "Authorization: Bearer $OWNER_TOKEN" -o /dev/null -w '%{http_code}\n'    # 204
   ```

   Seberapa cepat `watch` berhenti bergantung pada `nadi`, karena service memverifikasi token, bukan key:

   - **Feed perubahan akses aktif di `nadi`** (`NICE_SERVICE_API_KEY` terisi): pembacaan berikutnya dijawab 401 `token_stale`, klien membuktikan key lagi, NICE menjawab `invalid_client`, dan `watch` keluar dalam satu jeda. Ini yang teramati saat contoh diuji.
   - **Tanpa feed:** token yang sudah terbit tetap diterima sampai klien memperbaruinya, sekitar sepuluh menit setelah token terbit.

   Pada kedua jalur log memuat "NICE refused the API key for good", dan `watch` keluar dengan `FAILED: invalid_client` dan kode `1`.

4. **NICE mati sebentar.** Hentikan NICE saat `watch` berjalan: pembacaan berlanjut dengan token yang ada sampai token itu kedaluwarsa, lalu setiap putaran melaporkan `nice_unavailable` dan mencoba lagi. Setelah NICE hidup, `watch` pulih tanpa dijalankan ulang.

## Batas

- **Satu Service Account, satu service.** Tidak ada banyak akun dalam satu proses, dan tidak ada panggilan ke service `api_key` (lihat [`../shared-service-apikey`](../shared-service-apikey/README.md) untuk jalur proxy dan kunci per Service Account).
- **Token hanya di memori.** Setiap proses mengambil token sendiri; tidak ada penyimpanan ke disk dan tidak ada rotasi key otomatis.
- **Tanpa operasi ubah dan hapus**, dan tanpa atas nama user: token ini milik Service Account. Untuk bertindak atas nama user, lihat [`../agent-backend`](../agent-backend/README.md) (token exchange).
- **Batas laju.** NICE membatasi `/oauth/token` per alamat dan `client_id`, serta menghitung kegagalan API key per alamat. Jangan meminta token per request; pakai ulang seperti contoh ini.
