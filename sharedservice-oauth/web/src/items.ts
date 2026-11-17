import { ApiError } from "./api";

export interface Item {
  id: string;
  name: string;
  quantity: number;
  createdBy: string;
  actedBy?: string;
  createTime?: string;
  updateTime?: string;
}

export interface ItemInput {
  name: string;
  quantity: number;
}

export const ITEM_PERMISSIONS = {
  read: "inventaris.items.read",
  create: "inventaris.items.create",
  edit: "inventaris.items.edit",
  delete: "inventaris.items.delete",
} as const;

export type ItemAction = keyof typeof ITEM_PERMISSIONS;

export const NAME_MAX = 120;

export type ItemFormError = { field: "name" | "quantity"; message: string };

// validateItem mirrors the rules in the inventaris OpenAPI: name 1 to 120
// characters after trimming, quantity an integer of at least 0.
export function validateItem(name: string, quantity: string): { input: ItemInput } | ItemFormError {
  const trimmed = name.trim();
  if (!trimmed) return { field: "name", message: "Nama wajib diisi." };
  if ([...trimmed].length > NAME_MAX) return { field: "name", message: `Nama paling banyak ${NAME_MAX} karakter.` };
  if (!/^\d+$/.test(quantity.trim())) return { field: "quantity", message: "Jumlah harus bilangan bulat, 0 atau lebih." };
  const count = Number(quantity.trim());
  if (!Number.isSafeInteger(count)) return { field: "quantity", message: "Jumlah terlalu besar." };
  return { input: { name: trimmed, quantity: count } };
}

const VERBS: Record<ItemAction, string> = {
  read: "melihat daftar barang",
  create: "menambah barang",
  edit: "mengubah barang",
  delete: "menghapus barang",
};

// describeItemsError turns an answer of the backend into one sentence for the user.
export function describeItemsError(error: unknown, action: ItemAction): string {
  if (!(error instanceof ApiError)) {
    const reason = error instanceof Error ? error.message : String(error);
    return `Backend inventaris tidak dapat dipanggil: ${reason}. Pastikan backend berjalan.`;
  }
  switch (error.status) {
    case 400:
      return `Isian ditolak inventaris${error.description ? ` (${error.description})` : ""}. Periksa nama dan jumlahnya.`;
    case 401:
      return "Sesi Anda tidak dikenali lagi. Masuk lagi untuk melanjutkan.";
    case 403:
      if (error.code === "csrf") return "Permintaan ditolak karena token CSRF sesi tidak cocok. Muat ulang halaman.";
      return error.code === "access_denied"
        ? `Akses Anda ke project ini dicabut, jadi gagal ${VERBS[action]}. Operasi ini diperiksa langsung ke NICE.`
        : `Anda tidak punya izin untuk ${VERBS[action]} di project ini.`;
    case 404:
      return "Barang itu tidak ditemukan, mungkin sudah dihapus. Daftar dimuat ulang.";
    case 409:
      return error.code === "context_required"
        ? "Belum ada konteks aktif. Pilih project di panel Konteks."
        : `NICE menolak token untuk konteks aktif${error.description ? ` (${error.description})` : ""}. Pilih konteks lain.`;
    case 503:
      return "inventaris belum bisa memeriksa akses ke NICE. Coba lagi sebentar lagi.";
    default:
      return error.message;
  }
}
