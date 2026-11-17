import { type FormEvent, useId, useRef, useState } from "react";
import { type Item, type ItemFormError, validateItem } from "../items";
import { Dialog } from "./Dialog";

type Save = (input: { name: string; quantity: number }) => Promise<string | undefined>;

// Each submit answers undefined on success, or one sentence that stays in the dialog.
export function ItemFormDialog(props: { item?: Item; onSave: Save; onClose: () => void }) {
  const id = useId();
  const [name, setName] = useState(props.item?.name ?? "");
  const [quantity, setQuantity] = useState(String(props.item?.quantity ?? 0));
  const [fieldError, setFieldError] = useState<ItemFormError>();
  const [failure, setFailure] = useState<string>();
  const [pending, setPending] = useState(false);
  const nameRef = useRef<HTMLInputElement>(null);
  const quantityRef = useRef<HTMLInputElement>(null);
  const editing = props.item !== undefined;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const checked = validateItem(name, quantity);
    if ("field" in checked) {
      setFieldError(checked);
      (checked.field === "name" ? nameRef : quantityRef).current?.focus();
      return;
    }
    setFieldError(undefined);
    setFailure(undefined);
    setPending(true);
    const message = await props.onSave(checked.input);
    setPending(false);
    if (message) setFailure(message);
  };

  return (
    <Dialog title={editing ? "Ubah barang" : "Tambah barang"} onClose={props.onClose}>
      <form onSubmit={(event) => void submit(event)} noValidate>
        <div className="field">
          <label htmlFor={`${id}-name`}>Nama</label>
          <input
            id={`${id}-name`}
            ref={nameRef}
            value={name}
            maxLength={120}
            autoComplete="off"
            autoFocus
            onChange={(event) => setName(event.target.value)}
            aria-invalid={fieldError?.field === "name" || undefined}
            aria-describedby={fieldError?.field === "name" ? `${id}-name-error` : undefined}
          />
          {fieldError?.field === "name" && (
            <p id={`${id}-name-error`} className="field-error">
              {fieldError.message}
            </p>
          )}
        </div>
        <div className="field">
          <label htmlFor={`${id}-quantity`}>Jumlah</label>
          <input
            id={`${id}-quantity`}
            ref={quantityRef}
            value={quantity}
            inputMode="numeric"
            autoComplete="off"
            onChange={(event) => setQuantity(event.target.value)}
            aria-invalid={fieldError?.field === "quantity" || undefined}
            aria-describedby={fieldError?.field === "quantity" ? `${id}-quantity-error` : undefined}
          />
          {fieldError?.field === "quantity" && (
            <p id={`${id}-quantity-error`} className="field-error">
              {fieldError.message}
            </p>
          )}
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
            {pending ? "Menyimpan…" : editing ? "Simpan perubahan" : "Tambah barang"}
          </button>
        </div>
      </form>
    </Dialog>
  );
}

export function DeleteDialog(props: { item: Item; onConfirm: () => Promise<string | undefined>; onClose: () => void }) {
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
      title="Hapus barang?"
      description="Operasi sensitif: inventaris memeriksa akses Anda langsung ke NICE sebelum menghapus. Barang yang terhapus tidak bisa dikembalikan dari UI ini."
      onClose={props.onClose}
    >
      <p className="delete-target">
        <strong>{props.item.name}</strong>
        <span className="muted"> · {props.item.quantity} unit</span>
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
          {pending ? "Menghapus…" : "Hapus barang"}
        </button>
      </div>
    </Dialog>
  );
}
