export interface Friendly {
  title: string;
  hint: string;
}

export interface ErrorBody {
  code?: string;
  description?: string;
}

export function parseErrorBody(body: string): ErrorBody {
  try {
    const value = JSON.parse(body) as { error?: unknown; error_description?: unknown } | null;
    return {
      code: typeof value?.error === "string" ? value.error : undefined,
      description: typeof value?.error_description === "string" ? value.error_description : undefined,
    };
  } catch {
    return {};
  }
}

export function explainResponse(status: number, error: ErrorBody): Friendly | undefined {
  if (status < 400) return undefined;
  const marker = error.description ?? error.code;
  if (marker === "token_stale" || marker === "expired") {
    return {
      title: "Token basi",
      hint: "Service menolak token ini karena sudah tidak berlaku. SPA sudah mengambil token baru dan mengulang sekali; panggil lagi bila masih gagal.",
    };
  }
  if (marker === "permission_denied" || status === 403) {
    return {
      title: "Izin tidak cukup",
      hint: "Token sah, tetapi user belum punya izin untuk operasi ini di service. Minta peran yang sesuai di NICE.",
    };
  }
  if (status === 401) {
    return { title: "Token ditolak", hint: "Service tidak menerima token ini. Periksa bahwa aud token sama dengan slug service." };
  }
  if (status === 404) {
    return { title: "Jalur tidak ditemukan", hint: "Service berjalan, tetapi tidak punya jalur itu. Periksa path di VITE_SERVICES." };
  }
  if (status >= 500) {
    return { title: "Service gagal memproses", hint: "Service menjawab galat internal. Lihat log service." };
  }
  return { title: "Service menolak request", hint: "Lihat detail teknis di bawah untuk alasan dari service." };
}

export function explainTokenError(code: string | undefined, temporary: boolean): Friendly {
  if (code === "invalid_scope") {
    return {
      title: "Token service tidak diterbitkan",
      hint: "Service ini belum aktif di project, atau user tidak berhak atas scope-nya. Service lain tetap bisa dipanggil.",
    };
  }
  if (temporary) {
    return { title: "NICE belum menjawab", hint: "Penerbitan token gagal sementara. Coba lagi sebentar lagi; percobaan berikutnya menunggu backoff." };
  }
  return { title: "Token service tidak diterbitkan", hint: "NICE menolak permintaan token untuk service ini. Lihat detail teknis di bawah." };
}

export function explainCallError(): Friendly {
  return {
    title: "Service tidak terjangkau",
    hint: "Browser tidak bisa menghubungi service. Biasanya service belum berjalan, atau CORS-nya belum mengizinkan origin SPA ini.",
  };
}
