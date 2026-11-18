import { ApiError } from "./api";

export interface Device {
  id: string;
  name: string;
  serialNumber: string;
  status: DeviceStatus;
  location: string;
  createdBy: string;
  actedBy?: string;
  createTime?: string;
  updateTime?: string;
}

export const DEVICE_STATUSES = ["active", "inactive", "maintenance"] as const;

export type DeviceStatus = (typeof DEVICE_STATUSES)[number];

export const STATUS_LABEL: Record<DeviceStatus, string> = {
  active: "Aktif",
  inactive: "Nonaktif",
  maintenance: "Perawatan",
};

export interface DeviceInput {
  name: string;
  serialNumber: string;
  status: DeviceStatus;
  location: string;
}

export const DEVICE_PERMISSIONS = {
  read: "nadi.devices.read",
  create: "nadi.devices.create",
  edit: "nadi.devices.edit",
  delete: "nadi.devices.delete",
} as const;

export type DeviceAction = keyof typeof DEVICE_PERMISSIONS;

export const NAME_MAX = 120;
export const SERIAL_MAX = 64;
export const LOCATION_MAX = 120;

export type DeviceFormError = {
  field: "name" | "serialNumber" | "status" | "location";
  message: string;
};

export interface DeviceForm {
  name: string;
  serialNumber: string;
  status: string;
  location: string;
}

// validateDevice mirrors the rules in the nadi OpenAPI: name 1 to 120 and
// serial number 1 to 64 characters after trimming, a known status, and an
// optional location of at most 120 characters.
export function validateDevice(form: DeviceForm): { input: DeviceInput } | DeviceFormError {
  const name = form.name.trim();
  const serialNumber = form.serialNumber.trim();
  const location = form.location.trim();
  if (!name) return { field: "name", message: "Nama wajib diisi." };
  if ([...name].length > NAME_MAX)
    return {
      field: "name",
      message: `Nama paling banyak ${NAME_MAX} karakter.`,
    };
  if (!serialNumber) return { field: "serialNumber", message: "Nomor seri wajib diisi." };
  if ([...serialNumber].length > SERIAL_MAX)
    return {
      field: "serialNumber",
      message: `Nomor seri paling banyak ${SERIAL_MAX} karakter.`,
    };
  const status = DEVICE_STATUSES.find((value) => value === form.status);
  if (!status) return { field: "status", message: "Pilih status perangkat." };
  if ([...location].length > LOCATION_MAX)
    return {
      field: "location",
      message: `Lokasi paling banyak ${LOCATION_MAX} karakter.`,
    };
  return { input: { name, serialNumber, status, location } };
}

const VERBS: Record<DeviceAction, string> = {
  read: "melihat daftar perangkat",
  create: "menambah perangkat",
  edit: "mengubah perangkat",
  delete: "menghapus perangkat",
};

// describeDevicesError turns an answer of the backend into one sentence for the user.
export function describeDevicesError(error: unknown, action: DeviceAction): string {
  if (!(error instanceof ApiError)) {
    const reason = error instanceof Error ? error.message : String(error);
    return `Backend nadi tidak dapat dipanggil: ${reason}. Pastikan backend berjalan.`;
  }
  switch (error.status) {
    case 400:
      return `Isian ditolak nadi${error.description ? ` (${error.description})` : ""}. Periksa isiannya.`;
    case 401:
      return "Sesi Anda tidak dikenali lagi. Masuk lagi untuk melanjutkan.";
    case 403:
      if (error.code === "csrf") return "Permintaan ditolak karena token CSRF sesi tidak cocok. Muat ulang halaman.";
      return error.code === "access_denied"
        ? `Akses Anda ke project ini dicabut, jadi gagal ${VERBS[action]}. Operasi ini diperiksa langsung ke NICE.`
        : `Anda tidak punya izin untuk ${VERBS[action]} di project ini.`;
    case 404:
      return "Perangkat itu tidak ditemukan, mungkin sudah dihapus. Daftar dimuat ulang.";
    case 409:
      return error.code === "context_required"
        ? "Belum ada konteks aktif. Pilih project di panel Konteks."
        : `NICE menolak token untuk konteks aktif${error.description ? ` (${error.description})` : ""}. Pilih konteks lain.`;
    case 503:
      return "nadi belum bisa memeriksa akses ke NICE. Coba lagi sebentar lagi.";
    default:
      return error.message;
  }
}
