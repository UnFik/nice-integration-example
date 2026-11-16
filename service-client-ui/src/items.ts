import type { Claims } from "./jwt";
import type { TokenKey, TokenManager } from "./token-manager";

export interface Item {
  id: string;
  name: string;
  quantity: number;
  createdBy: string;
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

// ItemsApiError carries the {"error","error_description"} body of inventaris.
export class ItemsApiError extends Error {
  constructor(
    readonly status: number,
    readonly code?: string,
    readonly description?: string,
  ) {
    super(`inventaris menjawab HTTP ${status}${code ? ` ${code}` : ""}.`);
    this.name = "ItemsApiError";
  }
}

type Caller = Pick<TokenManager, "call">;

// itemsApi goes through tokens.call, so a 401 token_stale is retried once by the token manager.
export function itemsApi(tokens: Caller, baseUrl: string, target: TokenKey) {
  const send = async (path: string, method: string, body?: ItemInput): Promise<unknown> => {
    const response = await tokens.call(target, `${baseUrl}${path}`, {
      method,
      headers: { Accept: "application/json", ...(body ? { "Content-Type": "application/json" } : {}) },
      ...(body ? { body: JSON.stringify(body) } : {}),
    });
    if (response.status === 204) return undefined;
    const answer = (await response.json().catch(() => undefined)) as Record<string, unknown> | undefined;
    if (!response.ok) {
      throw new ItemsApiError(
        response.status,
        typeof answer?.error === "string" ? answer.error : undefined,
        typeof answer?.error_description === "string" ? answer.error_description : undefined,
      );
    }
    return answer;
  };
  const path = (id: string) => `/items/${encodeURIComponent(id)}`;
  return {
    async list(): Promise<Item[]> {
      const answer = (await send("/items", "GET")) as { items?: unknown } | undefined;
      if (!Array.isArray(answer?.items)) throw new ItemsApiError(200, "invalid_body");
      return answer.items.map(toItem);
    },
    async create(input: ItemInput): Promise<Item> {
      return toItem(await send("/items", "POST", input));
    },
    async update(id: string, input: ItemInput): Promise<Item> {
      return toItem(await send(path(id), "PUT", input));
    },
    async remove(id: string): Promise<void> {
      await send(path(id), "DELETE");
    },
  };
}

export function toItem(raw: unknown): Item {
  const value = (raw ?? {}) as Record<string, unknown>;
  const text = (field: unknown) => (typeof field === "string" && field ? field : undefined);
  return {
    id: typeof value.id === "string" ? value.id : "",
    name: typeof value.name === "string" ? value.name : "",
    quantity: typeof value.quantity === "number" ? value.quantity : 0,
    createdBy: typeof value.created_by === "string" ? value.created_by : "",
    createTime: text(value.create_time),
    updateTime: text(value.update_time),
  };
}

// permissionsOf reads perms from the claims shown by the UI; the service
// still decides, so this only explains a button that would be refused.
export function permissionsOf(claims: Claims | undefined): string[] | undefined {
  const perms = claims?.perms;
  return Array.isArray(perms) ? perms.filter((perm): perm is string => typeof perm === "string") : undefined;
}

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

// describeItemsError turns an answer of inventaris into one sentence for the user.
export function describeItemsError(error: unknown, action: ItemAction): string {
  if (!(error instanceof ItemsApiError)) {
    const reason = error instanceof Error ? error.message : String(error);
    return `inventaris tidak dapat dipanggil: ${reason}. Pastikan service berjalan dan CORS-nya mengizinkan origin UI ini.`;
  }
  switch (error.status) {
    case 400:
      return `Isian ditolak inventaris${error.description ? ` (${error.description})` : ""}. Periksa nama dan jumlahnya.`;
    case 401:
      return `Token ditolak inventaris${error.code ? ` (${error.code})` : ""}. Muat ulang halaman bila berulang.`;
    case 403:
      return error.code === "access_denied"
        ? `Akses Anda ke project ini dicabut, jadi gagal ${VERBS[action]}. Operasi ini diperiksa langsung ke NICE.`
        : `Anda tidak punya izin untuk ${VERBS[action]} di project ini.`;
    case 404:
      return "Barang itu tidak ditemukan, mungkin sudah dihapus. Daftar dimuat ulang.";
    case 503:
      return "inventaris belum bisa memeriksa akses ke NICE. Coba lagi sebentar lagi.";
    default:
      return error.message;
  }
}
