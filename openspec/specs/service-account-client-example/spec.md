# service-account-client-example Specification

## Purpose
Contoh pemanggil mesin (integrasi tanpa user) yang membuktikan API key Service Account ke NICE, lalu memanggil service `nice_token` `nadi` dengan token hasilnya, sebagai referensi mitra untuk integrasi server-ke-server.

## Requirements

### Requirement: API key hanya dikirim ke NICE
The client SHALL obtain its token from the NICE token endpoint with `client_credentials`, the Service Account id as `client_id`, the API key as `client_secret`, and scope `nice:aud:nadi`. It SHALL send the API key to no other address, and SHALL call `nadi` with `Authorization: Bearer <token>` only.

#### Scenario: Daftar perangkat
- **WHEN** the operator runs `list` with a valid Service Account id and API key
- **THEN** NICE receives one `client_credentials` request, `nadi` receives `GET /devices` with a Bearer token and no API key, and the devices are printed

#### Scenario: Service tidak pernah melihat key
- **WHEN** any command calls `nadi`
- **THEN** no request header, query or body sent to `nadi` contains the API key

### Requirement: Perintah daftar dan buat perangkat
The client SHALL offer `list`, which prints the devices of the Service Account's project, and `create`, which registers one device from a name, a serial number, a status and an optional location. Each SHALL exit with status 0 on success and non-zero on any failure.

#### Scenario: Membuat perangkat
- **WHEN** the operator runs `create` with a name, a serial number and status `active`
- **THEN** `nadi` receives `POST /devices` with those fields and the id of the created device is printed

#### Scenario: Masukan ditolak service
- **WHEN** `nadi` answers 400 to `create`
- **THEN** the client prints the error code and description of the answer and exits non-zero

### Requirement: Token dipakai ulang dan diperbarui
The client SHALL reuse one token while it is valid instead of asking NICE for a token per request. In `watch`, which lists the devices at a configurable interval until interrupted, it SHALL renew the token before it expires without interrupting the calls.

#### Scenario: Banyak panggilan, satu token
- **WHEN** `watch` lists the devices several times within the lifetime of one token
- **THEN** NICE receives one token request

#### Scenario: Token mendekati kedaluwarsa
- **WHEN** `watch` runs past the renewal time of its token
- **THEN** NICE receives a new token request and the next call to `nadi` carries the new token

#### Scenario: Dihentikan operator
- **WHEN** the operator interrupts `watch`
- **THEN** the client stops and exits with status 0

### Requirement: Token basi diulang satu kali
When `nadi` answers 401 with `token_stale` or `expired`, the client SHALL obtain a new token and send the request once more, and SHALL return the second answer whatever it is.

#### Scenario: Akses berubah lalu pulih
- **WHEN** `nadi` answers 401 `token_stale` to a request and NICE issues a new token
- **THEN** the client sends the request once more with the new token and prints its result

#### Scenario: Tetap ditolak
- **WHEN** `nadi` answers 401 `token_stale` to the repeated request as well
- **THEN** the client does not try a third time and exits non-zero naming `token_stale`

### Requirement: Key yang ditolak menghentikan klien
When NICE refuses the credentials for good (`invalid_client`), the client SHALL stop asking for tokens, SHALL explain that the key is wrong, revoked or expired, or the Service Account is disabled or does not own the key, and SHALL exit non-zero. `watch` SHALL NOT keep retrying a key that was refused for good.

#### Scenario: Key dicabut saat berjalan
- **WHEN** NICE answers `invalid_client` to a renewal during `watch`
- **THEN** `watch` stops, prints the explanation with the related checklist step, and exits non-zero

#### Scenario: Scope ditolak
- **WHEN** NICE answers `invalid_scope` to the token request
- **THEN** the client explains that `nadi` is not entitled to the organization or not active in the Service Account's project, and exits non-zero

### Requirement: Kegagalan sementara tidak menghentikan watch
When NICE or `nadi` cannot be reached or answers 5xx or 429, `list` and `create` SHALL exit non-zero naming which side failed. `watch` SHALL report the failure and try again at the next interval, and SHALL keep using a token that has not expired while NICE is down.

#### Scenario: NICE mati saat watch berjalan
- **WHEN** NICE stops answering while `watch` holds a token that has not expired
- **THEN** `watch` keeps listing devices with that token

#### Scenario: Service mati
- **WHEN** `nadi` cannot be reached during `watch`
- **THEN** `watch` reports that `nadi` is unreachable and tries again at the next interval

### Requirement: Izin kurang dijelaskan
When `nadi` answers 403, the client SHALL print the reason the service gave and SHALL point to the role binding of the Service Account, and SHALL exit non-zero without repeating the request.

#### Scenario: Role hanya baca
- **WHEN** the Service Account holds a role without `nadi.devices.create` and the operator runs `create`
- **THEN** the client prints `permission_denied`, names the missing permission and the role binding step, and exits non-zero

### Requirement: Rahasia tidak pernah tampil
The API key and any access token SHALL NOT appear in standard output, standard error, log lines, error messages, or the output of `scripts/smoke.sh`. Configuration SHALL be read from the environment and an ignored `.env` file, and `.env.example` SHALL hold placeholders only.

#### Scenario: Galat dari NICE
- **WHEN** a token request fails with any error
- **THEN** the printed message holds the error code and description only, and neither the API key nor a token

#### Scenario: Konfigurasi wajib kosong
- **WHEN** the issuer, the Service Account id or the API key is not set
- **THEN** the client exits non-zero before any network call and names the missing variable, not its value

### Requirement: Tes tanpa NICE dan smoke terhadap NICE
The unit tests SHALL run without a NICE or a `nadi` being up, against fakes of the token endpoint and of the device API. `scripts/smoke.sh` SHALL be the only automated check that touches a real NICE, SHALL read `.env` only, and SHALL name the README checklist step of each failure it can recognize.

#### Scenario: Tes unit
- **WHEN** `make test` runs on a machine with no NICE and no `nadi`
- **THEN** all tests pass

#### Scenario: Smoke dengan setup lengkap
- **WHEN** `scripts/smoke.sh` runs against a NICE and a `nadi` set up with the checklist
- **THEN** it lists the devices through the client and reports success without printing a key or a token

### Requirement: Panduan setup dan uji
The example SHALL carry `README.md` and `README.en.md` of equal content with the NICE version it needs, the revision it was tested against, a numbered setup checklist (a running `nadi`, a tenant Service Account in a project that activates `nadi`, a role binding with read and create permissions, an API key), the configuration variables, and a test checklist that includes revoking the key. The index README and `AGENTS.md` SHALL list the example.

#### Scenario: Mitra mengikuti README
- **WHEN** a partner follows the setup checklist and runs `create` then `list`
- **THEN** the created device is listed and its `created_by` is the Service Account id

#### Scenario: Indeks memuat contoh
- **WHEN** a reader opens the index README in either language
- **THEN** the table of examples, the flow diagram and the table of minimum NICE versions each include `service-account-client`
