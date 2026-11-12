# Design

## Context

- **`public-client-pkce`** sudah menyediakan pola yang terbukti: `oauth4webapi`, token manager dengan antrean refresh tunggal, scope dasar di setiap refresh (opsi B), pakai ulang token, pengulangan pada `token_stale`, dan NICE palsu yang meniru aturan subset dan penggantian scope grant.
- **Perilaku NICE yang dipakai:**

  | Change NICE | Yang dipakai contoh ini |
  |---|---|
  | `add-service-clients` | kelas service client, `nice:prj` wajib, organisasi bawaan = organisasi pertama yang meng-entitle service |
  | `add-delegated-nice-api` | `nice:aud:nice` dengan dependensi `nice` menghasilkan token terdelegasi; `GET /api/v1/me/contexts` (service = `nice_service` + dependensi); CORS dari redirect URI client `public` |
  | `add-seamless-org-switch` | `nice:org` saat refresh untuk service client; penolakan `access_denied` tanpa rotasi |

- **Repo contoh** menghindari kode bersama antarcontoh (`AGENTS.md`). Duplikasi kecil diterima.

## Goals / Non-Goals

**Goals:**
- Contoh berdiri sendiri: mitra bisa menyalin `service-client-ui/` tanpa membuka `public-client-pkce/`.
- Semua pilihan konteks berasal dari NICE (`/me/contexts`), tanpa tebakan dan tanpa input bebas.
- Pindah organisasi dan project terlihat jelas di panel jaringan browser: satu `POST /oauth/token`, tanpa redirect.

**Non-Goals:**
- Menulis ke API NICE.
- Banyak service: contoh hanya memanggil `inventaris`, walaupun `services` per project ditampilkan.
- Tab ganda (batasnya sama dengan contoh SPA).

## Decisions

### 1. Folder baru, bukan perluasan `public-client-pkce`

Client tenant dan service client berbeda konsep: organisasi tetap lawan multi-organisasi, `nice:prj` opsional lawan wajib, tanpa lawan dengan discovery. Menggabungkannya akan membuat contoh tenant penuh percabangan. Kode protokol (`oidc.ts`, token manager, NICE palsu) disalin lalu disesuaikan: kunci cache bertambah organisasi.

### 2. Token manager dengan organisasi di kunci

```text
kunci: "<org>|<service>|<project>"   (service "nice" tanpa project; token client tanpa service)
refresh(scope) selalu = BASE_SCOPE + tambahan per kunci
  inventaris @ (B, R):  BASE + " nice:org:<B>" (bila org grant berbeda) + " nice:aud:inventaris nice:prj:<R>"
  nice (konteks):       BASE + " nice:org:<org saat ini>" (bila berbeda) + " nice:aud:nice"
```

Organisasi aktif sesi disimpan di memori. Sumber kebenarannya adalah klaim `nice_org_id` dari token terakhir. Refresh tanpa `nice:org` tetap di organisasi terakhir, sesuai `organization-switch`.

### 3. Alur layar

```text
login -> token client -> userinfo (identitas)
      -> token nice terdelegasi -> /me/contexts -> pemilih
pilih konteks -> refresh inventaris @ konteks -> GET /items
galat refresh access_denied/invalid_scope -> tetap di konteks lama + muat ulang /me/contexts
```

### 4. Port dan CORS

Port 5181 (`strictPort`). `inventaris` menambah `http://localhost:5181` ke `CORS_ALLOWED_ORIGINS` bawaan. Di NICE, origin ini terbuka otomatis lewat redirect URI client `public` (`add-delegated-nice-api`), jadi `NICE_CORS_ALLOWED_ORIGINS` tidak perlu diubah.

### 5. NICE palsu untuk tes

Disalin dari contoh SPA dan diperluas dengan:
- `nice:org` saat refresh (tolak bila user bukan anggota);
- `/api/v1/me/contexts` yang hanya menerima token `aud=nice` dengan `perms` memuat `nice.me.contexts.read`;
- penolakan `invalid_scope` untuk project yang tidak aktif.

Tes membuktikan pindah organisasi tanpa redirect, penolakan tanpa merusak sesi, dan pemilih yang hanya berisi konteks dari jawaban.

## Risks / Trade-offs

- **[Contoh bergantung pada tiga change NICE yang belum dibangun]** → Tes otomatis memakai NICE palsu. Uji manual baru dijalankan setelah ketiga change digabung, dan README mencatat versi NICE minimum.
- **[Bentuk jawaban `/me/contexts` bisa berubah saat backend diimplementasikan]** → Tipe respons dibaca dari swagger NICE terbaru saat contoh dikerjakan, dan NICE palsu mengikuti swagger itu.
- **[Duplikasi kode protokol dengan `public-client-pkce`]** → Disengaja demi kemandirian contoh. Perbaikan protokol harus diterapkan ke keduanya, dan `AGENTS.md` mencatatnya.

## Migration Plan

Tidak ada migrasi. Penyesuaian contoh lain hanya README dan nilai bawaan env.
