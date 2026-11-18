import { type FormEvent, useId, useRef, useState } from "react";
import {
  DEVICE_STATUSES,
  type Device,
  type DeviceForm,
  type DeviceFormError,
  type DeviceInput,
  LOCATION_MAX,
  NAME_MAX,
  SERIAL_MAX,
  STATUS_LABEL,
  validateDevice,
} from "../devices";
import { Dialog } from "./Dialog";

type Save = (input: DeviceInput) => Promise<string | undefined>;

type FieldName = DeviceFormError["field"];

// Each submit answers undefined on success, or one sentence that stays in the dialog.
export function DeviceFormDialog(props: { device?: Device; onSave: Save; onClose: () => void }) {
  const id = useId();
  const [form, setForm] = useState<DeviceForm>({
    name: props.device?.name ?? "",
    serialNumber: props.device?.serialNumber ?? "",
    status: props.device?.status ?? "active",
    location: props.device?.location ?? "",
  });
  const [fieldError, setFieldError] = useState<DeviceFormError>();
  const [failure, setFailure] = useState<string>();
  const [pending, setPending] = useState(false);
  const refs = useRef<Partial<Record<FieldName, HTMLInputElement | HTMLSelectElement | null>>>({});
  const editing = props.device !== undefined;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const checked = validateDevice(form);
    if ("field" in checked) {
      setFieldError(checked);
      refs.current[checked.field]?.focus();
      return;
    }
    setFieldError(undefined);
    setFailure(undefined);
    setPending(true);
    const message = await props.onSave(checked.input);
    setPending(false);
    if (message) setFailure(message);
  };

  const invalid = (field: FieldName) =>
    fieldError?.field === field ? { "aria-invalid": true, "aria-describedby": `${id}-${field}-error` } : {};
  const error = (field: FieldName) =>
    fieldError?.field === field && (
      <p id={`${id}-${field}-error`} className="field-error">
        {fieldError.message}
      </p>
    );
  const set = (field: FieldName, value: string) => setForm((current) => ({ ...current, [field]: value }));

  return (
    <Dialog title={editing ? "Ubah perangkat" : "Tambah perangkat"} onClose={props.onClose}>
      <form onSubmit={(event) => void submit(event)} noValidate>
        <div className="field">
          <label htmlFor={`${id}-name`}>Nama</label>
          <input
            id={`${id}-name`}
            ref={(node) => void (refs.current.name = node)}
            value={form.name}
            maxLength={NAME_MAX}
            autoComplete="off"
            autoFocus
            onChange={(event) => set("name", event.target.value)}
            {...invalid("name")}
          />
          {error("name")}
        </div>
        <div className="field">
          <label htmlFor={`${id}-serialNumber`}>Nomor seri</label>
          <input
            id={`${id}-serialNumber`}
            ref={(node) => void (refs.current.serialNumber = node)}
            value={form.serialNumber}
            maxLength={SERIAL_MAX}
            autoComplete="off"
            onChange={(event) => set("serialNumber", event.target.value)}
            {...invalid("serialNumber")}
          />
          {error("serialNumber")}
        </div>
        <div className="field">
          <label htmlFor={`${id}-status`}>Status</label>
          <select
            id={`${id}-status`}
            ref={(node) => void (refs.current.status = node)}
            value={form.status}
            onChange={(event) => set("status", event.target.value)}
            {...invalid("status")}
          >
            {DEVICE_STATUSES.map((status) => (
              <option key={status} value={status}>
                {STATUS_LABEL[status]}
              </option>
            ))}
          </select>
          {error("status")}
        </div>
        <div className="field">
          <label htmlFor={`${id}-location`}>Lokasi (opsional)</label>
          <input
            id={`${id}-location`}
            ref={(node) => void (refs.current.location = node)}
            value={form.location}
            maxLength={LOCATION_MAX}
            autoComplete="off"
            onChange={(event) => set("location", event.target.value)}
            {...invalid("location")}
          />
          {error("location")}
        </div>
        {failure && (
          <p className="callout error" role="alert">
            {failure}
          </p>
        )}
        <div className="dialog-actions">
          <button type="button" onClick={props.onClose}>
            Batal
          </button>
          <button type="submit" className="primary" disabled={pending}>
            {pending ? "Menyimpan…" : editing ? "Simpan perubahan" : "Tambah perangkat"}
          </button>
        </div>
      </form>
    </Dialog>
  );
}

export function DeleteDialog(props: { device: Device; onConfirm: () => Promise<string | undefined>; onClose: () => void }) {
  const [failure, setFailure] = useState<string>();
  const [pending, setPending] = useState(false);
  const confirm = async () => {
    setFailure(undefined);
    setPending(true);
    const message = await props.onConfirm();
    setPending(false);
    if (message) setFailure(message);
  };
  return (
    <Dialog
      title="Hapus perangkat?"
      description="Operasi sensitif: nadi memeriksa akses Anda langsung ke NICE sebelum menghapus. Perangkat yang terhapus tidak bisa dikembalikan dari UI ini."
      onClose={props.onClose}
    >
      <p className="delete-target">
        <strong>{props.device.name}</strong>
        <span className="muted"> · {props.device.serialNumber}</span>
      </p>
      {failure && (
        <p className="callout error" role="alert">
          {failure}
        </p>
      )}
      <div className="dialog-actions">
        <button type="button" autoFocus onClick={props.onClose}>
          Batal
        </button>
        <button type="button" className="danger" disabled={pending} onClick={() => void confirm()}>
          {pending ? "Menghapus…" : "Hapus perangkat"}
        </button>
      </div>
    </Dialog>
  );
}
