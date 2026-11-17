## REMOVED Requirements

### Requirement: Login sebagai service client
**Reason**: UI SPA public client diganti pola backend-for-frontend; login dilakukan backend sebagai service client confidential.
**Migration**: Lihat requirement "Login lewat backend sebagai service client confidential" di `service-bff-example`.

### Requirement: Konteks dari NICE
**Reason**: Penemuan konteks lewat refresh `nice:aud:nice` dari browser ditolak NICE untuk service client dan tidak lagi dilakukan UI.
**Migration**: Lihat requirement "Konteks dari NICE lewat backend" di `service-bff-example` (token exchange atas nama pengguna sesi).

### Requirement: Pindah konteks lewat refresh
**Reason**: Refresh dan pindah konteks pindah dari browser ke backend.
**Migration**: Lihat requirement "Pindah konteks lewat refresh di backend" di `service-bff-example`.

### Requirement: Token per konteks
**Reason**: Browser tidak lagi memegang token; backend memegang refresh token dan cache token per konteks lewat `niceclient`.
**Migration**: Lihat requirement "Sesi sisi server" dan "Pindah konteks lewat refresh di backend" di `service-bff-example`.

### Requirement: Panduan setup dan uji
**Reason**: Folder `service-client-ui/` dihapus dan setupnya berubah (client confidential, tanpa deklarasi dependensi, tanpa form konteks manual).
**Migration**: Lihat requirement "Panduan setup dan uji" di `service-bff-example`.
