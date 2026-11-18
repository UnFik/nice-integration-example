# sharedservice-oauth/web

[English](README.en.md)

UI React untuk service `nadi`. UI ini hanya bicara ke backend di [`../backend`](../backend) lewat origin yang sama: tidak memegang token, tidak memanggil NICE, dan tidak memuat pustaka OAuth. Login, refresh, pindah organisasi dan project, serta penemuan konteks dijalankan backend. Panduan setup NICE dan checklist uji ada di [`../README.md`](../README.md).

## Isi

| Berkas | Isi |
| --- | --- |
| `src/api.ts` | Klien `/auth/session`, `/auth/contexts`, `/auth/context`, `/auth/logout`, dan `/devices*`: URL relatif, cookie sesi (`credentials: "same-origin"`), dan `X-CSRF-Token` pada setiap request tulis |
| `src/session.ts` | Status halaman: sesi, konteks aktif dan `perms`, pilihan konteks, pindah konteks, perangkat, keluar, serta galat `login_error`, 401 `session_ended`, 409 `context_refused`, dan 503 saat keluar |
| `src/devices.ts` | Validasi isian perangkat dan kalimat galat |
| `src/ui/`, `src/App.tsx` | Halaman masuk, bilah atas dengan konteks aktif, panel Konteks, tabel perangkat, dan dialog tambah/ubah/hapus. Tombol mengikuti `perms` dari `GET /auth/session` |

Tombol Masuk adalah navigasi halaman ke `/auth/login` (backend mengalihkan ke NICE), bukan `fetch`.

## Menjalankan

```bash
npm install
npm run dev      # http://localhost:5181, /auth dan /devices diteruskan ke backend di 127.0.0.1:8081
npm test         # Vitest dengan backend palsu (src/test/fake-backend.ts), tanpa backend atau NICE
npm run typecheck
npm run build    # web/dist, disajikan backend di 8081
```

Proxy Vite tidak mengubah header `Origin` (`changeOrigin: false`), jadi backend melihat `http://localhost:5181`. Nilai itu harus sama dengan `PUBLIC_URL` backend saat dev, karena pemeriksaan CSRF membandingkannya.
