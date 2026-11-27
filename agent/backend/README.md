# agent/backend (`agen`)

[English](README.en.md)

Contoh backend agen AI: MCP server (transport Streamable HTTP) untuk service NICE `agen`, ditulis dalam Go dengan [SDK MCP Go resmi](https://github.com/modelcontextprotocol/go-sdk). Agen tidak menulis daftar tool per service: ia menanyakan ke NICE service mana yang boleh dipanggil user di project mana, beserta operasinya (*caller service discovery*), lalu memanggil operasi itu **atas nama user** yang sedang memakai agen, dengan token hasil token exchange (RFC 8693):

- token user hanya dikirim ke endpoint token NICE, tidak pernah ke service lain (tanpa *token passthrough*);
- tidak ada Service Account lintas organisasi: izin token tujuan adalah izin user di project tujuan, tanpa batas tambahan dari NICE (lihat "Batas izin");
- agen tidak menyimpan refresh token user;
- service tujuan melihat agen sebagai aktor (`act.sub`), dan `nadi` mencatatnya sebagai `acted_by`;
- operasi yang ditandai sensitif di katalog hanya dikirim setelah user menyetujuinya di MCP client.

```text
MCP client --Bearer token user (aud=agen)--> agen /mcp
agen --token exchange (aud=nice)--> NICE /oauth/token, lalu GET /api/v1/me/services dan /me/services/nadi/operations
agen --token exchange: subject_token + client agen-backend (aud=nadi)--> NICE /oauth/token
agen --Bearer token hasil exchange (act.sub=agen-backend)--> <endpoint nadi di katalog>/devices
```

Versi NICE minimum: `235445c` (`add-caller-service-discovery`), karena tool memakai `GET /api/v1/me/services` dan `GET /api/v1/me/services/{slug}/operations`; NICE yang lebih lama menjawab `404` dan tool menjawab `discovery_unavailable`. Sebelumnya `b361d25` (`service-calls-without-service-account`), karena penyelesaian `perms_ref` meneruskan token request, bukan API key. Isian operasi yang kaya, `get_operation_schema`, dan pembersihan `x-secret` butuh NICE dengan `add-operation-schemas` (`dev`); terhadap NICE yang lebih lama isian itu kosong, `get_operation_schema` menjawab `schemas_unavailable`, dan `call_operation` meneruskan jawaban apa adanya. SDK: `github.com/ABA-Developer/nice-auth-sdk` `v0.2.5` (`Services`, `ServiceOperationsIfChanged`, `OperationSchemas`, `RedactSecrets`, `CallOperation`). SDK MCP dipin di `go.mod` pada `v1.8.0`, yang memuat protokol MCP `2026-07-28` untuk persetujuan tanpa sesi.

Diuji terhadap nice revisi: skema operasi (2026-11-26) dengan tes unit (NICE dan model palsu), lalu di VPS dengan NICE `dev-abd6ec4`, image `agen:bfe23e0`, dan katalog `nadi` yang diimpor ulang dengan `acted_by` bertanda `x-secret`: `scripts/smoke.sh` lulus (empat tool, `get_operation_schema` `listDevices` dengan `input_schema` dan empat status, `call_operation` 20 perangkat), hasil `call_operation` lewat MCP tidak memuat `acted_by`, dan di dashboard chat model membuat perangkat dengan status dari `enum` `list_operations`, hasil `createDevice` dan `listDevices` tanpa `acted_by` sementara `nadi` tetap menyimpan `acted_by` client agen, lalu `get_operation_schema` `updateDevice` menjelaskan field wajib dan bentuk jawabannya. Model tidak memanggil `get_operation_schema` sendiri sebelum membuat perangkat; isian `list_operations` sudah cukup baginya. Sebelumnya: dashboard chat (2026-11-26) dengan tes unit (NICE, `nadi`, model, dan PostgreSQL palsu atau sementara), lalu di VPS dengan NICE `dev-235445c`, image `agen:99c97d0`, PostgreSQL, dan model OpenAI-compatible `deepseek-v4.1-flash:netra`, lewat HTTP dengan cookie jar seperti browser: masuk lewat `/auth/login` ke NICE, POST tanpa CSRF `403`, daftar perangkat dan buat perangkat lewat chat (`acted_by` agen), hapus dengan Tolak (`not_approved`) lalu Setujui (`204`, jawaban kedua `409`), restart container (sesi berakhir, riwayat tetap dari PostgreSQL), dan keluar. Smoke MCP dan uji persetujuan MCP juga lulus pada image itu. Lalu di browser sungguhan (viewport 1280×800): masuk lewat halaman login NICE, kirim pesan dari UI yang membuat lalu meminta hapus perangkat, kartu persetujuan dengan pesan katalog, Setujui (perangkat terhapus), semua request halaman hanya ke origin agen, `localStorage` dan `sessionStorage` kosong, cookie tidak terbaca skrip, dan Keluar lewat menu akun (revoke dan `end_session` di NICE, `/auth/session` lalu `401`). Pindah konteks hanya diuji di tes unit, karena user demo hanya punya satu project yang memakai `nadi`. Sebelumnya: SDK `v0.2.4` dan `nice` `235445c`, 2026-11-25, dengan tes unit (NICE dan `nadi` palsu), lalu `scripts/smoke.sh` dan uji hapus dengan persetujuan (dua putaran protokol `2026-07-28`) terhadap NICE `dev-235445c` di VPS, dengan endpoint `nadi` `https`.

## Struktur folder

Polanya meniru `nice/internal/modules` (lihat `nice/AGENTS.md`), dipangkas untuk satu service kecil: resource `call` (discovery dan pemanggilan service lain atas nama user), `chat` (dashboard), dan `session` (login dashboard), tanpa facade.

```text
cmd/agent/main.go                    tipis: memanggil bootstrap.Run
docs/                                openapi.json (ditulis tangan) + paket kecil yang meng-embed-nya; tes OpenAPI dan README
migrations/                          SQL riwayat percakapan (di-embed, diterapkan saat start bila DATABASE_URL diisi)
internal/
  domain/                            aturan murni: slug, izin, target, endpoint yang aman, pesan dan judul chat, sesi (*_domain.go)
  app/call/
    usecase/                         ListServices, ListOperations, CallOperation (satu *_usecase.go per fungsi), Service,
                                     kontrak Discoverer dan Operator (call_contract.go)
    dto/                             Caller, peta service, operasi, persetujuan, Failure (*_dto.go)
    internal/{key,failure,approval}/ validasi ID, pemetaan galat ke Failure, state persetujuan bertanda tangan (*_helper.go)
  app/chat/
    usecase/                         RunTurn, AnswerApproval, percakapan (list, get, create, rename, delete); kontrak
                                     Conversations, Model, Tools (chat_contract.go)
    dto/                             percakapan, pesan dalam bentuk netral, ToolCall, ToolResult, Event
    internal/turn/                   kunci satu giliran per percakapan, system prompt, bentuk riwayat untuk model
  app/session/                       sesi dashboard (login, konteks, keluar), disalin dari sharedservice-oauth
  adapter/
    mcp/                             rute HTTP dan rantai niceauth/verify (routes.go), server MCP dan tabel tool yang juga
                                     dibaca chat (tools_routes.go), satu handler per tool (*_handler.go)
    niceexchange/                    pembungkus niceauth: Services, ServiceOperations, CallOperation (*_client.go)
    nice/                            login, konteks, dan userinfo dashboard lewat niceauth/credentials (*_gateway.go)
    rest/auth/                       /auth/* dan middleware sesi /chat/ (cookie, CSRF, token konteks, agen.mcp.use)
    rest/chat/                       API chat /chat/*, respons giliran sebagai server-sent events
    rest/web/                        penyaji web/dist dengan fallback SPA
    llm/{anthropic,openaicompat}/    model chat: Messages API dan Chat Completions yang kompatibel OpenAI (*_client.go)
    memory/, postgres/               penyimpanan sesi dan percakapan (*_repository.go); chattest/ tes kontrak bersama
  platform/{config,postgres}/        env dan .env; pool PostgreSQL dan migrasi
  bootstrap/                         perakitan; tes integrasi dengan NICE palsu
  arch_test.go                       tes arsitektur: akhiran nama berkas dan arah impor
scripts/smoke.sh                     uji smoke terhadap NICE yang sudah diatur
```

Alasannya: `main.go` hanya merakit; aturan dan use case tidak mengimpor `net/http`, SDK MCP, `niceauth/credentials`, atau `niceauth/verify` (adapter yang mengimpor, ditegakkan `internal/arch_test.go`); setiap berkas membawa lapisannya di akhiran nama (`_domain`, `_usecase`, `_contract`, `_dto`, `_handler`, `_routes`, `_client`, `_gateway`, `_repository`, `_middleware`, `_helper`, `_mapper`); satu `*_usecase.go` berisi satu fungsi bernama sama dengan berkasnya. `openapi.json` tetap ditulis tangan dan di-embed dari `docs/`; `docs/openapi_test.go` menjaga bahwa izin di dokumen sama dengan yang ditegakkan kode.

## Endpoint dan tool

| Endpoint | Akses |
|---|---|
| `POST /mcp` | token NICE `aud=agen` dengan izin `agen.mcp.use` |
| `GET /.well-known/oauth-protected-resource` (juga `.../mcp`) | terbuka: `resource` `<PUBLIC_URL>/mcp`, `authorization_servers` issuer NICE, `scopes_supported` scope dasar dan `nice:aud:agen` |
| `GET /healthz`, `GET /openapi.json` | terbuka, tidak ada di dokumen OpenAPI |
| `GET /auth/login`, `GET /auth/callback`, `GET /auth/session`, `GET /auth/contexts`, `POST /auth/context`, `POST /auth/logout` | login dashboard (cookie `agen_session`, `HttpOnly`, `SameSite=Lax`): masuk lewat NICE, sesi dan token CSRF, daftar organisasi dan project, pindah konteks lewat satu refresh, keluar dengan revoke refresh token. Tulis butuh header `X-CSRF-Token` dan `Origin` sama dengan `PUBLIC_URL`. Tidak ada di dokumen OpenAPI |
| `/chat/*` | API chat dashboard: hanya cookie sesi, header `Authorization` dijawab `401`. Token konteks aktif diperiksa seperti `/mcp` dan harus memegang `agen.mcp.use` (`403` bila tidak); tulis butuh CSRF seperti `/auth/*`. Tidak ada di dokumen OpenAPI |
| `GET /` | dashboard (`WEB_DIR`) bila hasil build-nya ada; rute lain tanpa ekstensi mendapat `index.html` |

`/mcp` hanya menerima Bearer dan mengabaikan cookie sesi dashboard. Tanpa token, `/mcp` dijawab `401` dengan `WWW-Authenticate: Bearer resource_metadata="<PUBLIC_URL>/.well-known/oauth-protected-resource"`. Token untuk service lain dijawab `401` `wrong_audience`, token tanpa `agen.mcp.use` dijawab `403` `permission_denied`.

| Tool | Argumen | Jawaban |
|---|---|---|
| `list_services` | tanpa argumen | organisasi user, project-nya, dan slug service yang boleh dipanggil user di tiap project, beserta nama dan deskripsi service. Organisasi yang project-nya hanya sebagian disebut NICE ditandai `projects_truncated` |
| `list_operations` | `service` | operasi service itu yang izinnya dipegang user: `key`, `method`, `path`, `summary`, `description`, `permission`, `request_fields` (nama, lokasi `path`/`query`/`body`, wajib atau tidak, `type`, `description`, `format`, `enum`, `enum_truncated`; isian yang tidak dinyatakan dokumen service tidak muncul), dan `sensitive` |
| `get_operation_schema` | `service`, `operations` (1–10 key) | per key: `input_schema` (JSON Schema `path`, `query`, `header`, dan `body` bersarang) dan `responses` (per status: `description`, `headers`, `schema`) dari NICE, `truncated` bila NICE memotong skema, dan `missing` untuk key yang tidak boleh dipanggil user |
| `call_operation` | `service`, `operation` (key), `organization_id`, `project_id`, `path_params`, `query`, `body` | `status` dan `body` JSON jawaban service |

Daftar tool sama untuk setiap user; apa yang boleh dipanggil dibaca dari NICE saat tool berjalan, dari role user saat itu, bukan dari klaim token. Agen tidak menyimpan peta. Daftar operasi terakhir per user dan service disimpan di memori (paling banyak 1000) beserta `ETag`-nya, tetapi setiap pembacaan tetap bertanya ke NICE dengan `If-None-Match`: simpanan hanya dipakai saat NICE menjawab `304`, dan dibuang saat NICE menolak.

`call_operation` mencocokkan `service` dengan peta dan `operation` dengan daftar operasi pada setiap panggilan. Alamat (endpoint dan base path katalog), method, dan header operasi tidak pernah diambil dari argumen model. Sebelum exchange untuk service tujuan, agen menolak dengan `invalid_argument`:

- `service` yang bukan slug, atau `organization_id`/`project_id` yang tidak berbentuk `org_...`/`prj_...` (nilai itu menjadi scope);
- organisasi lain tanpa `project_id`, atau project yang tidak disebut peta untuk service itu di organisasi tersebut (kecuali NICE menandai daftar project organisasi itu `projects_truncated`);
- key operasi yang tidak ada, nama `query` yang bukan field `query` operasi itu;
- parameter path yang hilang, tidak dikenal, kosong, `.`, `..`, atau memuat `/`, `\`, atau karakter kontrol (diperiksa SDK).

Tanpa `organization_id` dan `project_id`, exchange hanya meminta `nice:aud:<service>`, dan NICE memakai organisasi dan project token user. `body` dikirim apa adanya sebagai JSON; service tujuan yang memeriksa isinya, dan `nadi` menolak field yang tidak dikenal.

### Persetujuan operasi sensitif

Token hasil exchange membawa izin user di project tujuan, sama seperti token service client milik user itu sendiri (`nice/docs/api.md`, "Token exchange: backend atas nama user"). NICE tidak memotongnya untuk aktor: bila user memegang `nadi.devices.delete`, token yang dipegang agen juga memegangnya. Karena daftar tool tidak lagi membatasi operasi, batas yang ditambah agen adalah persetujuan: operasi yang ditandai `sensitive` di katalog (di `nadi`: `DELETE /devices/{id}`) hanya dikirim setelah user menyetujuinya.

Server MCP berjalan tanpa sesi, jadi agen tidak bisa mengirim permintaan elicitation sendiri ke client. Ia memakai pola multi round-trip protokol MCP `2026-07-28` (SEP-2322):

1. Panggilan pertama `call_operation` untuk operasi sensitif dijawab `resultType: input_required` dengan `inputRequests.approve` (elicitation form: pesan yang menyebut service, ringkasan operasi, method dan path terisi, organisasi, dan project; skema `{approve: boolean}`) dan `requestState`.
2. MCP client menampilkan formulir ke user, lalu memanggil tool yang sama lagi dengan argumen yang sama, `inputResponses.approve` (`action: accept`, `approve: true`), dan `requestState`.
3. Agen mengirim operasi itu sekali. `decline`, `cancel`, atau `approve: false` dijawab `not_approved`, tanpa exchange untuk service tujuan dan tanpa panggilan.

`requestState` adalah state buram bertanda tangan HMAC yang terikat ke user dan isi panggilan persis (service, operasi, konteks, parameter, query, body), berlaku 5 menit, dan hanya sekali pakai. State untuk user lain, panggilan lain, yang diubah, kedaluwarsa, atau sudah dipakai membuat agen meminta persetujuan baru. Kuncinya dibuat acak saat proses mulai, jadi restart membatalkan persetujuan yang tertunda.

MCP client protokol lama, atau yang tidak menyatakan elicitation form, tidak bisa ditanya: operasi sensitif dijawab `approval_unavailable` dan tidak dikirim. Operasi lain tetap berjalan. Operasi tulis yang tidak sensitif (mis. `POST /devices`) tidak meminta persetujuan; admin katalog yang menentukan operasi mana yang sensitif. Untuk uji, beri user `nadi.editor` (tanpa izin hapus) atau `nadi.admin` untuk mencoba persetujuan.

### Mengapa hanya service `nice_token`

Service `api_key` memakai kunci per Service Account dan menolak token user (`403` `service_account_required`), jadi agen yang bertindak atas nama user hanya memanggil service `nice_token`. NICE tidak memasukkan service `api_key` ke peta dan menjawab `404` untuk operasinya, jadi contoh service `api_key` di repo ini, `pengiriman`, dijawab `service_not_allowed`. Memanggil service `api_key` lewat Service Account milik agen akan menghilangkan identitas user (tidak ada lagi `created_by` user di service tujuan) dan bukan pola contoh ini.

### Cara kerja panggilan

`list_services` dan `list_operations` memanggil `credentials.ExchangeClient.Services` dan `ServiceOperations` dengan token user dari request MCP. Keduanya berbagi satu token API NICE terdelegasi (`aud=nice`) per token user, jadi discovery hanya menambah satu exchange per token user, bukan per service atau project. `call_operation` membaca peta, operasi, dan skema operasi yang dipanggil (tiga `GET` ke NICE), lalu memanggil `CallOperation`, yang mengirim lewat `DoAs` dengan kunci `{Org, Project, Audience}`. `DoAs`:

- menyimpan token per `(user, organisasi, project, service)` sampai 60 detik sebelum kedaluwarsa, dan tidak pernah memakai token yang umurnya melebihi token user;
- menjalankan satu exchange per kunci untuk panggilan paralel, tanpa antrean antar kunci, sehingga konteks (organisasi atau project) yang berbeda ditukar bersamaan;
- mengulang exchange dan request sekali bila service menjawab `401` `token_stale` atau `expired`.

**Field rahasia.** Sebelum jawaban service sampai ke model, riwayat percakapan, atau pesan galat, agen membuang setiap properti yang skema status jawaban itu tandai `x-secret: true` (`RedactSecrets` dengan `ResponseSchema(status)`). Skema dibaca sebelum panggilan dikirim: bila NICE gagal sementara, panggilan tidak dikirim (`nice_unavailable` atau `temporarily_unavailable`), supaya tidak ada efek samping yang jawabannya tidak bisa dibersihkan. Bila NICE belum menyajikan skema (`404`), panggilan tetap dikirim dan jawabannya diteruskan apa adanya.

Endpoint katalog harus `https`, kecuali hostnya loopback: agen menolak endpoint lain dengan `insecure_endpoint` sebelum token apa pun dikirim ke sana, sama dengan aturan SDK.

Transport MCP berjalan *stateless* dengan jawaban JSON: setiap `POST /mcp` berdiri sendiri, sehingga `curl` bisa memanggil tool dalam satu request dan panggilan paralel dari satu client tidak saling menunggu.

### Galat

Kegagalan dikembalikan sebagai hasil tool bertanda galat (`isError: true`) berisi kode, alasan singkat dari NICE atau service bila ada, dan penjelasan. Token, secret, dan body mentah tidak pernah masuk hasil maupun log.

| Kode | Sumber | Arti |
|---|---|---|
| `discovery_unavailable` | NICE | NICE belum punya caller service discovery: naikkan NICE |
| `service_not_allowed` | NICE, agen | user tidak boleh memanggil service itu di project mana pun, atau service itu bukan `nice_token` |
| `token_stale_after_upgrade` | NICE | token terdelegasi diterbitkan sebelum NICE di-upgrade (`403 delegation_not_allowed`): ambil token user baru |
| `insecure_endpoint` | agen | endpoint katalog service itu `http` ke host non-loopback |
| `approval_unavailable` | agen | operasi sensitif di MCP client yang tidak bisa meminta persetujuan |
| `not_approved` | agen | user menolak atau menutup persetujuan |
| `access_denied` | exchange | user bukan anggota aktif organisasi itu, organisasi tidak aktif, atau tidak meng-entitle service tujuan atau `agen` |
| `invalid_scope` | exchange | service tujuan tidak aktif, atau project tidak aktif di organisasi itu atau tidak mengaktifkan service tujuan |
| `invalid_grant` (`subject_token_expiring`) | exchange | token user tinggal kurang dari 60 detik: perbarui token di MCP client |
| `invalid_grant` (`token_stale`, `subject_invalid`) | exchange | akses user berubah, token bukan token user untuk `agen`, atau sesi berakhir: login lagi |
| `invalid_client`, `unauthorized_client` | exchange | client agen ditolak: periksa `NICE_CLIENT_ID` dan `NICE_CLIENT_SECRET` |
| `nice_unavailable` | exchange | NICE tidak menjawab |
| `insufficient_scope` (`permission_denied`), `permission_denied` | service | user tidak memegang izin operasi itu di project tersebut |
| `invalid_argument` | agen, service | argumen tidak cocok dengan peta atau operasi (lihat "Endpoint dan tool"); field perangkat tidak sah (`nadi`) |

Contoh teks hasil tool untuk organisasi yang bukan milik user:

```text
access_denied (organization is not usable): NICE refused a token for nadi in that organization: the user is not an active member of it, the organization is not active, or it does not use nadi or agen
```

## Dashboard chat

Selain `/mcp`, agen menyajikan dashboard chat ([`../web`](../web/README.md)) di `/`. User masuk lewat `/auth/login` dengan client `agen-backend` (langkah 7). Refresh token disimpan di server, dan browser hanya memegang cookie `agen_session`. `/chat/*` hanya menerima cookie itu: request yang membawa `Authorization` ditolak `401`, sebaliknya `/mcp` mengabaikan cookie.

**Satu giliran.** `POST /chat/conversations/{id}/messages` dengan `{"text"}` (paling panjang 8000 karakter) menyimpan pesan user, lalu memanggil model dengan system prompt, riwayat percakapan, dan empat tool yang sama dengan server MCP (nama, deskripsi, dan schema input dibaca dari satu tabel di `adapter/mcp/tools_routes.go`). Tool dijalankan lewat use case yang sama dengan MCP, atas nama user, dengan token konteks aktif sebagai subject token exchange. Model tidak pernah melihat token. Setiap langkah disimpan dulu, baru dikirim sebagai server-sent event:

| Event | Data |
|---|---|
| `assistant` | pesan model (teks dan panggilan tool) |
| `tool_call` | `call_id`, `name`: tool mulai berjalan |
| `tool_result` | pesan berisi hasil satu tool (JSON yang sama dengan `structuredContent` MCP, atau `{"error","message"}`) |
| `approval` | `call_id`, `message`: panggilan menunggu persetujuan |
| `done` | `reason`: `answered`, `awaiting_approval`, atau `step_limit` |
| `error` | `error`, `message`: mis. `model_unavailable`, `model_refused` |

Giliran tetap berjalan bila browser menutup koneksi; dashboard membaca ulang percakapan setelahnya. Satu percakapan hanya menjalankan satu giliran sekaligus (giliran kedua dijawab `409 turn_in_progress`), dan satu giliran berhenti setelah `CHAT_MAX_STEPS` panggilan model. Tanpa `LLM_PROVIDER`, riwayat tetap bisa dibaca, tetapi mengirim pesan dijawab `503 chat_not_configured`.

**Persetujuan.** Saat `call_operation` mengenai operasi sensitif, panggilan disimpan dengan status `awaiting_approval`, beserta state persetujuan dan pesan dari katalog (bukan dari teks model), lalu giliran berhenti. Dashboard menampilkan pesan itu dengan tombol Setujui dan Tolak, yang mengirim `POST /chat/conversations/{id}/approvals/{call_id}` dengan `{"approve": true|false}`. Panggilan diulang dengan state tersimpan, jadi aturannya sama dengan MCP: hanya user yang sama, sekali pakai, dan berlaku 5 menit. Tolak tidak mengirim apa pun dan tidak menukar token untuk service tujuan. State yang kedaluwarsa, atau hilang karena agen restart (kuncinya per proses), meminta persetujuan baru. Pesan baru yang dikirim saat panggilan masih menunggu menolak panggilan itu.

**Riwayat.** Percakapan milik satu user (`sub` token sesi). User lain mendapat `404`. Dengan `DATABASE_URL`, riwayat disimpan di PostgreSQL, di dua tabel: `conversations` (`id`, `user_sub`, `title`, `created_at`, `updated_at`) dan `messages` (`conversation_id`, `seq`, `role` `user`/`assistant`/`tool`, `parts` jsonb dalam bentuk netral, sehingga ganti provider tidak mengubah data). Menghapus percakapan menghapus pesannya. Hasil tool disimpan apa adanya, karena itu data user dari service yang ia panggil sendiri. Token, refresh token, dan kunci persetujuan tidak pernah disimpan.

**Risiko.**
- Hasil tool berasal dari service dan bisa memuat teks yang mencoba mengarahkan model (prompt injection). Operasi sensitif tetap menunggu persetujuan dengan pesan dari katalog, operasi lain dibatasi izin user di NICE, dan batas langkah membatasi satu giliran.
- Tidak ada kuota atau batas biaya LLM per user. Pasang batas di provider bila dashboard dibuka untuk banyak user.
- Sesi disimpan di memori, sama seperti `nadi`: restart berarti masuk lagi, tetapi riwayat di PostgreSQL tetap ada.

## Prasyarat

- SDK diambil dari modul publik `github.com/ABA-Developer/nice-auth-sdk`, jadi contoh ini bisa dibangun tanpa repo `nice`. Kodenya butuh SDK `v0.2.5` atau sesudahnya. Repo `nice` di sebelah `example/` hanya dibutuhkan untuk menjalankan NICE lokal dan `nice/scripts/dev-token.sh`.
- Go 1.27, `curl`, `jq`, dan `openssl` (untuk `nice/scripts/dev-token.sh`).
- NICE dengan versi minimum di atas, bawaannya di `http://localhost:8080`.
- `nadi` berjalan dan sudah diatur sampai langkah 8 [`../sharedservice-oauth/backend/README.md`](../sharedservice-oauth/backend/README.md) (daftar, impor, aktifkan, entitlement, aktivasi di project, binding). Endpoint `nadi` di katalog adalah alamat yang dipanggil agen, jadi harus `https` atau loopback (lokal: `http://127.0.0.1:8081`). User uji memegang role `nadi.editor` di project yang sama, atau `nadi.admin` untuk mencoba persetujuan hapus. Jangan memakai `nadi.operator`: itu role operator platform service, bukan role project.
- Untuk uji organisasi lain: user uji juga anggota organisasi kedua, dengan entitlement `nadi`, project yang mengaktifkan `nadi`, dan binding `nadi.editor` di sana (langkah 6 sampai 8 README `nadi` untuk organisasi itu).

## Konfigurasi

Salin `.env.example` ke `.env`. Agen membaca `.env` di direktori tempat ia dijalankan; variabel di environment menang atas berkas.

| Variabel | Bawaan | Arti |
|---|---|---|
| `NICE_ISSUER` | wajib | issuer NICE, sama persis dengan klaim `iss` token |
| `NICE_CLIENT_ID` | wajib | `client_id` client confidential `agen-backend` (langkah 7) |
| `NICE_CLIENT_SECRET` | wajib | secret client itu. Hanya dikirim ke endpoint token NICE |
| `LISTEN_ADDR` | `127.0.0.1:8083` | alamat listen |
| `PUBLIC_URL` | `http://127.0.0.1:8083` | alamat yang dipakai MCP client dan browser dashboard; `resource` metadata adalah `<PUBLIC_URL>/mcp`, dan client `agen-backend` harus punya redirect URI `<PUBLIC_URL>/auth/callback`. Dev dengan Vite (`npm run dev` di `../web`): `http://localhost:5182` |
| `JWKS_FILE` | `data/jwks.json` | salinan JWKS di disk (`data/` tidak di-commit) |
| `POST_LOGOUT_URL` | `<PUBLIC_URL>/` | ke mana NICE mengirim browser setelah Keluar di dashboard; harus post-logout redirect URI client `agen-backend`. `none` membiarkan browser di halaman NICE |
| `SESSION_IDLE_TIMEOUT` | `360h` | sesi dashboard yang tidak dipakai selama ini berakhir (durasi Go; `360h` berarti 15 hari) |
| `WEB_DIR` | `../web/dist` | hasil build dashboard (`npm run build` di `../web`), disajikan di `/` bila direktorinya ada |
| `DATABASE_URL` | kosong | PostgreSQL riwayat percakapan, mis. `postgres://agen:<password>@localhost:5432/agen?sslmode=disable`; migrasi diterapkan saat start. Kosong: riwayat di memori dan hilang saat restart |
| `LLM_PROVIDER` | kosong | `anthropic` atau `openai` (API Chat Completions yang kompatibel OpenAI: OpenAI, Ollama, vLLM). Kosong: chat nonaktif, `/mcp` tetap jalan |
| `LLM_MODEL` | `claude-sonnet-5-5` untuk `anthropic` | model; wajib untuk `openai` |
| `LLM_BASE_URL` | bawaan provider | alamat API provider, mis. `http://localhost:11434/v1` untuk Ollama |
| `LLM_API_KEY` | kosong | key provider, rahasia, hanya dikirim ke provider. Wajib untuk `anthropic` |
| `CHAT_MAX_STEPS` | `8` | batas panggilan model dalam satu giliran chat (1 sampai 50) |

Variabel wajib yang kosong menghentikan start dengan pesan yang menyebut namanya. Tidak ada alamat service di konfigurasi: agen memanggil setiap service di endpoint katalog NICE.

Penyelesaian `perms_ref` (pemanggil yang izin `agen`-nya tidak muat di token) selalu menyala tanpa konfigurasi: agen meneruskan token request itu sendiri ke NICE (`POST /api/v1/permission-resolutions` dengan `Authorization: Bearer`). Agen tidak memegang kredensial NICE selain service client `agen-backend`. Bila NICE tidak terjangkau atau menolak token itu, panggilan dijawab `503`.

## Menjalankan dan tes

```bash
make run          # go run ./cmd/agent di 127.0.0.1:8083
make test         # go test -race; tidak butuh NICE (NICE dan nadi palsu)
make smoke        # scripts/smoke.sh, butuh NICE yang sudah diatur dengan checklist di bawah
```

Bila `go` di `PATH` tidak cocok dengan `GOROOT`, panggil `make GO=/path/ke/go test`.

Tes menjalankan agen lewat HTTP di dalam proses dengan client MCP dari SDK, di depan NICE palsu yang meniru aturan token exchange dan discovery. Tes yang menjaga pola: service palsu gagal bila pernah menerima token `aud=agen` (tanpa passthrough), sepuluh panggilan dalam satu konteks menghasilkan satu exchange untuk `nadi` dan satu untuk API NICE, exchange untuk dua konteks (organisasi atau project) yang berbeda berjalan bersamaan, dan operasi sensitif terkirim tepat sekali setelah disetujui dan tidak sama sekali tanpa persetujuan.

### Menjalankan di container

`Dockerfile` ada di `agent/` (konteks build-nya folder `agent/`, jadi dijalankan dari sini sebagai `docker build -t agen ..`). Ia membangun dashboard (`../web`) dengan Node 22, membangun binary statis dengan Go 1.27, lalu menjalankan binary itu di image distroless `nonroot`. Image mengisi `LISTEN_ADDR=0.0.0.0:8083`, `JWKS_FILE=/data/jwks.json`, dan `WEB_DIR=/web` (dashboard di `/`, MCP di `/mcp`, satu host); `.env` tidak ikut masuk image, jadi konfigurasi diberikan lewat environment.

```bash
docker build -t agen ..
docker run --rm -p 8083:8083 -v agen-data:/data \
  -e NICE_ISSUER=https://nice.example.com \
  -e PUBLIC_URL=https://agen.example.com -e NICE_CLIENT_ID -e NICE_CLIENT_SECRET agen
```

- `PUBLIC_URL` harus alamat `https` publik yang dipakai MCP client: metadata resource menyebut `<PUBLIC_URL>/mcp`, dan endpoint `agen` di NICE harus menunjuk alamat yang sama.
- `NICE_ISSUER` dan endpoint katalog setiap service harus `https` kecuali host loopback: SDK menolak `http` ke host lain, dan di dalam container `127.0.0.1` adalah container itu sendiri. Endpoint katalog yang hanya berlaku di jaringan Docker (mis. `http://nadi:8081`) dijawab `insecure_endpoint`.
- Secret diteruskan dari environment shell (`-e NICE_CLIENT_SECRET` tanpa nilai), tidak ditulis di baris perintah.

## Checklist setup NICE

Jalankan dari direktori `example/agent/backend`, dalam satu sesi shell. Langkah console disebut dengan nama halamannya; potongan `curl` adalah padanannya. Password dibaca dari terminal dan tidak pernah ditulis ke berkas.

```bash
export NICE=${NICE:-http://localhost:8080}   # sama dengan NICE_ISSUER di .env; dev-token.sh juga membacanya

read -rp 'Email Platform Admin: ' ADMIN_EMAIL; read -rsp 'Password: ' ADMIN_PASSWORD; echo
ADMIN_TOKEN=$(../../nice/scripts/dev-token.sh "$ADMIN_EMAIL" "$ADMIN_PASSWORD"); unset ADMIN_PASSWORD
read -rp 'Email owner organisasi tenant: ' OWNER_EMAIL; read -rsp 'Password: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD"); unset OWNER_PASSWORD

ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .organization.id)
PROJECT_ID=prj_...   # project tenant tempat nadi aktif
USER_ID=usr_...      # user uji yang memakai agen
PLATFORM_ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $ADMIN_TOKEN" | jq -r .organization.id)
```

Token berlaku 30 menit; ambil ulang bila `curl` dijawab 401.

### 1. Env NICE

Sama dengan contoh service lain: `nice/.env` memuat `NICE_OUTBOUND_ALLOW_HTTP=true` dan `NICE_OUTBOUND_ALLOW_CIDRS=127.0.0.0/8`, lalu NICE dijalankan ulang. Tanpa keduanya NICE menolak endpoint `http://127.0.0.1:8083`.

### 2. Daftarkan `agen`

Console: Platform Admin, **Shared Service**, **Daftarkan Shared Service**. Mode bawaan `nice_token`.

```bash
curl -sS "$NICE/api/v1/shared-services" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"slug":"agen","name":"Agen","endpoint":"http://127.0.0.1:8083","health_path":"/healthz"}' | jq '.shared_service | {slug, status}'
```

### 3. Impor `docs/openapi.json`

Console: detail service `agen`, impor katalog. Dokumen diambil dari berkas, karena agen baru bisa berjalan setelah client-nya ada (langkah 7).

```bash
PREVIEW=$(jq -n --rawfile doc docs/openapi.json '{document: $doc}' | curl -sS "$NICE/api/v1/shared-services/agen/catalog-imports" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d @-)
jq '{added: [.diff.added[]? | {key, permission}], roles: [.diff.roles[]? | .key]}' <<<"$PREVIEW"
curl -sS "$NICE/api/v1/shared-services/agen/catalog-imports/$(jq -r .import_id <<<"$PREVIEW")/apply" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d "$(jq '{hash}' <<<"$PREVIEW")"
```

Pratinjau menunjukkan satu operasi `mcp` (`agen.mcp.use`) dan role `agen.admin` serta `agen.user` (dari `x-nice-roles`). Tidak ada `agen.viewer`, karena dokumen tidak punya operasi baca.

### 4. Aktifkan service

```bash
curl -sS "$NICE/api/v1/shared-services/agen/activate" -X POST -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.shared_service | {slug, status}'
```

### 5. Project di organisasi platform

Service client harus milik project organisasi platform. Pakai project yang ada, atau buat satu (console: Platform Admin, organisasi platform, **Project**).

```bash
AGENT_PROJECT_ID=$(curl -sS "$NICE/api/v1/organizations/$PLATFORM_ORG_ID/projects" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Agen","slug":"agen"}' | jq -r .project.id)
```

### 6. Client `agen-user` (public)

Client untuk user dan MCP client memperoleh token `aud=agen`. Console: project itu, **OAuth client**, lalu ikat ke `agen` (butuh `nice.platform.catalog.manage`). Redirect URI di bawah milik MCP Inspector; `dev-token.sh` memakai yang pertama.

```bash
CLIENT=$(curl -sS "$NICE/api/v1/projects/$AGENT_PROJECT_ID/clients" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"agen-user","client_type":"CLIENT_TYPE_PUBLIC",
       "redirect_uris":["http://localhost:6274/oauth/callback","http://localhost:6274/oauth/callback/debug"],
       "grant_types":["authorization_code","refresh_token"]}')
USER_CLIENT_ID=$(jq -r .oauth_client.client_id <<<"$CLIENT")
curl -sS "$NICE/api/v1/projects/$AGENT_PROJECT_ID/clients/$(jq -r .oauth_client.id <<<"$CLIENT")/service" -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"agen"}' \
  | jq '.oauth_client | {client_class, service}'
# {"client_class":"service","service":"agen"}
```

### 7. Client `agen-backend` (confidential)

Aktor token exchange, sekaligus client login dashboard (backend-for-frontend). Exchange menolak client `public`, dan memisahkan dua client menunjukkan bahwa secret hanya ada di server agen. Untuk dashboard, client ini butuh:

- redirect URI `<PUBLIC_URL>/auth/callback` (bawaan `http://127.0.0.1:8083/auth/callback`; dev dengan Vite: `http://localhost:5182/auth/callback`);
- post-logout redirect URI `<PUBLIC_URL>/` (atau nilai `POST_LOGOUT_URL`);
- grant `authorization_code` dan `refresh_token`: refresh token disimpan di server agen, dan pindah konteks dilakukan lewat refresh.

Untuk client `agen-backend` yang sudah ada (dibuat hanya dengan redirect `.../unused`), tambahkan URI dan grant di atas di console (detail client) atau lewat `PUT` client yang sama.

```bash
CLIENT=$(curl -sS "$NICE/api/v1/projects/$AGENT_PROJECT_ID/clients" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"agen-backend","client_type":"CLIENT_TYPE_CONFIDENTIAL",
       "redirect_uris":["http://127.0.0.1:8083/auth/callback","http://localhost:5182/auth/callback"],
       "post_logout_redirect_uris":["http://127.0.0.1:8083/","http://localhost:5182/"],
       "grant_types":["authorization_code","refresh_token"]}')
curl -sS "$NICE/api/v1/projects/$AGENT_PROJECT_ID/clients/$(jq -r .oauth_client.id <<<"$CLIENT")/service" -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"agen"}' \
  | jq '.oauth_client | {client_class, service}'
cp -n .env.example .env && chmod 600 .env
sed -i '/^NICE_CLIENT_ID=/d; /^NICE_CLIENT_SECRET=/d' .env
jq -r '"NICE_CLIENT_ID=" + .oauth_client.client_id, "NICE_CLIENT_SECRET=" + .client_secret' <<<"$CLIENT" >> .env
unset CLIENT
```

Secret hanya tampil sekali dan langsung masuk `.env` tanpa dicetak. Bila hilang, putar secret client itu di console lalu ganti `NICE_CLIENT_SECRET`.

### 8. Organisasi user: entitlement, aktivasi, binding

Login lewat service client hanya selesai di organisasi yang meng-entitle `agen`. Exchange ke organisasi lain memeriksa hal yang sama ditambah entitlement service tujuan, tetapi tidak meminta `agen` aktif di project organisasi itu. Console: Platform Admin, **Organisasi** (entitlement), lalu project tenant, **Product & Activation** dan **IAM**.

```bash
curl -sS "$NICE/api/v1/organizations/$ORG_ID/entitlements" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"agen"}'
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/services/agen/activation" -X PUT -H "Authorization: Bearer $OWNER_TOKEN"
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/role-bindings" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_USER\",\"principal_id\":\"$USER_ID\",\"role\":\"agen.user\"}" | jq -r .role_binding.id
```

IAM menyalin katalog dan aktivasi secara asinkron: bila binding dijawab `unknown_role` atau `service_not_active`, tunggu beberapa detik lalu ulangi. Untuk uji organisasi lain, beri organisasi kedua entitlement `agen` juga.

### 9. Jalankan agen

```bash
make run
curl -sS http://127.0.0.1:8083/healthz
```

Biarkan berjalan di terminal lain.

### 10. Token user

Token user untuk agen diminta lewat client `agen-user` dengan `nice:aud:agen` dan project tempat user memegang `agen.user`. `nice:org` memastikan login masuk ke organisasi itu.

```bash
read -rp 'Email user uji: ' USER_EMAIL; read -rsp 'Password: ' USER_PASSWORD; echo
USER_TOKEN=$(CLIENT_ID=$USER_CLIENT_ID REDIRECT=http://localhost:6274/oauth/callback \
  ../../nice/scripts/dev-token.sh "$USER_EMAIL" "$USER_PASSWORD" "" "openid nice:org:$ORG_ID nice:aud:agen nice:prj:$PROJECT_ID")
unset USER_PASSWORD
export AGENT_USER_TOKEN=$USER_TOKEN   # dibaca scripts/smoke.sh
```

### 11. Smoke

```bash
make smoke
```

Skrip membaca `.env` dan `AGENT_USER_TOKEN` dari environment, tidak pernah mencetak token atau secret, dan memeriksa: NICE menjawab, `/healthz` menjawab, `/mcp` tanpa token dijawab `401` dengan `resource_metadata`, metadata menyebut `NICE_ISSUER`, lalu dengan token user `tools/list` memuat empat tool, `list_services` memuat `nadi`, `list_operations` untuk `nadi` memuat `GET /devices`, `get_operation_schema` operasi itu memuat `input_schema`, dan `call_operation` operasi itu sukses. `AGENT_URL` mengarahkan skrip ke agen di alamat lain (mis. `AGENT_URL=https://agen.example.com make smoke`). Kegagalan dipetakan ke langkah:

| Jawaban | Langkah yang kemungkinan terlewat |
|---|---|
| `.env` tidak ada, agen tidak terjangkau | 7, 9 |
| `/mcp` dengan token dijawab `401` `wrong_audience` | 10: token harus diminta dengan `nice:aud:agen` lewat `agen-user` |
| `/mcp` dijawab `403` `permission_denied` | 8: binding `agen.user` |
| tool dijawab `discovery_unavailable` | NICE lebih lama dari versi minimum |
| tool dijawab `service_not_allowed`, atau `list_services` tanpa `nadi` | langkah 6 sampai 8 README `nadi` untuk user dan project itu |
| tool dijawab `insecure_endpoint` | endpoint `nadi` di katalog bukan `https` atau loopback |
| tool dijawab `invalid_client` | 7: `NICE_CLIENT_ID` atau `NICE_CLIENT_SECRET` |
| tool dijawab `invalid_scope` | `nadi` belum aktif di project token (langkah 7 README `nadi`) |
| tool dijawab `invalid_grant` | 10: token user hampir habis, basi, atau sesinya berakhir; ambil token baru |

## Checklist uji

Dengan `USER_TOKEN` dari langkah 10. Fungsi `tool` memanggil satu tool dalam satu request:

```bash
tool() {   # tool <nama> [argumen JSON]
  jq -nc --arg name "$1" --argjson args "${2:-"{}"}" \
    '{jsonrpc: "2.0", id: 1, method: "tools/call", params: {name: $name, arguments: $args}}' |
  curl -sS http://127.0.0.1:8083/mcp -H "Authorization: Bearer $USER_TOKEN" \
    -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -d @- |
  jq '.result | {isError, structuredContent, text: .content[0].text}'
}
```

1. **Daftar tool.** Empat tool: `list_services`, `list_operations`, `get_operation_schema`, `call_operation`:

   ```bash
   curl -sS http://127.0.0.1:8083/mcp -H "Authorization: Bearer $USER_TOKEN" -H 'Content-Type: application/json' \
     -H 'Accept: application/json, text/event-stream' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq '[.result.tools[].name]'
   ```

2. **Discovery.** Peta memuat `nadi` di project user; operasi `nadi` memuat `GET /devices` dan `DELETE /devices/{id}` dengan `sensitive: true` (bila user memegang izin hapus):

   ```bash
   tool list_services
   tool list_operations '{"service":"nadi"}' | jq '.structuredContent.operations[] | {key, method, path, sensitive}'
   ```

   Catat key operasi `POST /devices`, `GET /devices`, dan `DELETE /devices/{id}` sebagai `CREATE`, `LIST`, dan `DELETE`.

3. **Perangkat dibuat agen.** `created_by` adalah user dan `acted_by` adalah `client_id` `agen-backend` (`NICE_CLIENT_ID`):

   ```bash
   tool call_operation "{\"service\":\"nadi\",\"operation\":\"$CREATE\",
     \"body\":{\"name\":\"Sensor Suhu\",\"serial_number\":\"SN-2026-0001\",\"status\":\"active\",\"location\":\"Gudang A\"}}"
   tool call_operation "{\"service\":\"nadi\",\"operation\":\"$LIST\"}"
   ```

4. **Organisasi lain milik user.** Dengan organisasi kedua (`ORG_B_ID`, project `PROJECT_B_ID`) yang memenuhi prasyarat dan meng-entitle `agen`:

   ```bash
   tool call_operation "{\"service\":\"nadi\",\"operation\":\"$LIST\",\"organization_id\":\"$ORG_B_ID\",\"project_id\":\"$PROJECT_B_ID\"}"
   ```

   Sukses, dengan perangkat organisasi itu. Project yang tidak disebut `list_services` untuk `nadi` dijawab `invalid_argument` tanpa exchange.

5. **Hapus dengan persetujuan.** Dengan user yang memegang `nadi.devices.delete` dan `DEVICE_ID` dari langkah 3. Panggilan pertama tanpa header protokol `2026-07-28` dijawab `approval_unavailable` dan tidak menghapus apa pun:

   ```bash
   tool call_operation "{\"service\":\"nadi\",\"operation\":\"$DELETE\",\"path_params\":{\"id\":\"$DEVICE_ID\"}}"
   ```

   Client protokol `2026-07-28` mengirim header `MCP-Protocol-Version`, `Mcp-Method`, `Mcp-Name`, dan `_meta` dengan kapabilitas elicitation di setiap request. Putaran pertama mengembalikan pesan persetujuan dan `requestState`; putaran kedua mengirim jawaban user bersama state itu:

   ```bash
   approve() {   # approve <argumen JSON> [state] [true|false]
     jq -nc --argjson args "$1" --arg state "${2:-}" --argjson ok "${3:-true}" '
       {jsonrpc: "2.0", id: 1, method: "tools/call", params: ({name: "call_operation", arguments: $args,
         _meta: {"io.modelcontextprotocol/protocolVersion": "2026-07-28",
                 "io.modelcontextprotocol/clientInfo": {name: "curl", version: "1"},
                 "io.modelcontextprotocol/clientCapabilities": {elicitation: {form: {}}}}}
         + (if $state == "" then {} else {requestState: $state,
             inputResponses: {approve: {action: "accept", content: {approve: $ok}}}} end))}' |
     curl -sS http://127.0.0.1:8083/mcp -H "Authorization: Bearer $USER_TOKEN" -H 'Content-Type: application/json' \
       -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2026-07-28' \
       -H 'Mcp-Method: tools/call' -H 'Mcp-Name: call_operation' -d @-
   }
   ARGS="{\"service\":\"nadi\",\"operation\":\"$DELETE\",\"path_params\":{\"id\":\"$DEVICE_ID\"}}"
   FIRST=$(approve "$ARGS"); jq '.result | {resultType, message: .inputRequests.approve.params.message}' <<<"$FIRST"
   approve "$ARGS" "$(jq -r .result.requestState <<<"$FIRST")" true | jq '.result | {isError, structuredContent}'
   ```

   Putaran kedua dijawab `status: 204` dan perangkat hilang dari `$LIST`. Mengirim state yang sama sekali lagi, atau state itu dengan `DEVICE_ID` lain, dijawab dengan permintaan persetujuan baru; `false` dijawab `not_approved`.

6. **Organisasi bukan milik user.** Hasilnya galat, dan `nadi` tidak dipanggil: `invalid_argument` bila organisasi itu tidak ada di peta, atau `access_denied` dari exchange:

   ```bash
   tool call_operation "{\"service\":\"nadi\",\"operation\":\"$LIST\",\"organization_id\":\"$OTHER_ORG_ID\",\"project_id\":\"$OTHER_PROJECT_ID\"}"
   ```

7. **Audit NICE.** Setiap exchange tercatat `token.exchange` dengan pelaku user dan `details` `client_id`, `service`, dan `project_id`, termasuk exchange untuk API NICE (`service` `nice`). Exchange yang sukses tercatat di organisasi tujuan. Penolakan tercatat di organisasi token user, dengan `details.reason` (mis. `not_member`, `invalid_scope`) dan `details.target_organization_id`:

   ```bash
   curl -sS "$NICE/api/v1/audit/records?action=token.exchange&page_size=10" -H "Authorization: Bearer $OWNER_TOKEN" \
     | jq '.records[] | {actor_id, outcome, project_id, details}'
   ```

   Agen menyimpan token hasil exchange, jadi banyak panggilan tool dalam satu konteks hanya menghasilkan satu record per konteks, ditambah satu untuk API NICE.

8. **MCP Inspector (opsional).** `npx @modelcontextprotocol/inspector`, transport Streamable HTTP ke `http://127.0.0.1:8083/mcp`. NICE tidak menyediakan Dynamic Client Registration, jadi isi `client_id` `agen-user` secara manual dan tambahkan `nice:prj:<project>` ke scope; scope dari metadata saja tidak cukup untuk token service client. NICE tanpa dev mode menolak redirect `http` milik client public, jadi Inspector hanya bisa login ke NICE lokal yang berjalan dalam dev mode. Langkah ini tidak termasuk uji integrasi contoh ini.

## Batas

- **Tanpa DCR.** Client MCP didaftarkan manual (langkah 6).
- **Tanpa service `api_key`.** Service itu hanya melayani Service Account (lihat "Mengapa hanya service `nice_token`"), jadi contoh ini tidak menunjukkan pemanggilan service `api_key`.
- **Persetujuan hanya di client protokol `2026-07-28`.** Client lama tidak bisa menjalankan operasi sensitif (`approval_unavailable`).
- **Persetujuan per proses.** Set nonce sekali pakai disimpan di memori: dengan beberapa replika, satu persetujuan bisa dipakai lagi dalam 5 menit di replika lain untuk panggilan yang persis sama. Restart membatalkan persetujuan yang tertunda.
- **Skema tidak divalidasi di agen.** Model membaca `get_operation_schema`, tetapi agen tidak memeriksa `body` terhadap `input_schema`; service tujuan yang memeriksanya. Operasi tetap dipanggil lewat satu tool generik, bukan satu tool per operasi.
- **Satu project per panggilan.** Pertanyaan lintas banyak project berarti satu `call_operation` per project, masing-masing satu exchange; `/oauth/token` dibatasi per client (bawaan 30 per menit).
- Tanpa UI chat atau LLM, dan tanpa pekerjaan agen saat user tidak hadir.
- **Tanpa feed perubahan akses di agen.** Token user yang basi tetap ditolak di exchange (`invalid_grant` `token_stale`), karena NICE memeriksa feed sendiri.
