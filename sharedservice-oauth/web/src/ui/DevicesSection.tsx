import { useState } from "react";
import { DEVICE_PERMISSIONS, type Device, type DeviceAction, type DeviceInput, STATUS_LABEL } from "../devices";
import type { Session, SessionState } from "../session";
import { absoluteTime, relativeTime, shortId } from "./format";
import { Icon } from "./icons";
import { DeleteDialog, DeviceFormDialog } from "./DeviceDialogs";
import { useNow } from "./useNow";

const ACTION_LABEL = { create: "Tambah", edit: "Ubah", delete: "Hapus" } as const;

type Dialog = { kind: "add" } | { kind: "edit"; device: Device } | { kind: "delete"; device: Device };

export function DevicesSection(props: {
  session: Session;
  state: SessionState;
  notify: (kind: "success" | "error", text: string) => void;
}) {
  const { session, state, notify } = props;
  const [dialog, setDialog] = useState<Dialog>();
  const now = useNow(30_000);
  // A context whose token the backend could not get lists nothing until another is chosen.
  const project = state.contextError ? undefined : state.active?.project;

  const can = (action: DeviceAction): boolean => state.perms.includes(DEVICE_PERMISSIONS[action]);
  const missing = (["create", "edit", "delete"] as const).filter((action) => !can(action));
  const why = (action: DeviceAction) => `Token konteks ini tidak memuat ${DEVICE_PERMISSIONS[action]}.`;

  const save = (device: Device | undefined) => async (input: DeviceInput) => {
    const result = await session.saveDevice(input, device?.id);
    if (!result.ok) return result.message;
    setDialog(undefined);
    notify("success", device ? `Perangkat "${input.name}" diperbarui.` : `Perangkat "${input.name}" ditambahkan.`);
    return undefined;
  };

  const remove = (device: Device) => async () => {
    const result = await session.removeDevice(device.id);
    if (!result.ok) return result.message;
    setDialog(undefined);
    notify("success", `Perangkat "${device.name}" dihapus.`);
    return undefined;
  };

  const devices = state.devices;
  return (
    <section className="card" aria-labelledby="perangkat-title" aria-busy={state.devicesLoading || undefined}>
      <header className="card-head">
        <div>
          <h2 id="perangkat-title">Perangkat</h2>
          {project && devices && (
            <p className="muted small" aria-live="polite">
              {devices.length} perangkat
            </p>
          )}
        </div>
        {project && (
          <div className="actions">
            <button type="button" className="quiet" disabled={state.devicesLoading} onClick={() => void session.loadDevices()}>
              <Icon name="refresh" />
              Muat ulang
            </button>
            <button
              type="button"
              className="primary"
              data-action="add"
              disabled={!can("create")}
              title={can("create") ? undefined : why("create")}
              onClick={() => setDialog({ kind: "add" })}
            >
              <Icon name="plus" />
              Tambah perangkat
            </button>
          </div>
        )}
      </header>

      {project && devices && missing.length > 0 && (
        <p className="callout info perm-note">
          <Icon name="info" />
          <span>
            {`${missing.map((action) => ACTION_LABEL[action]).join(", ")} dinonaktifkan: token konteks ini tidak memuat ${missing.map((action) => DEVICE_PERMISSIONS[action]).join(", ")}.`}
          </span>
        </p>
      )}

      {!project ? (
        <div className="empty">
          <Icon name="box" />
          <h3>{state.contextError ? "Konteks aktif tidak dapat dipakai" : "Belum ada konteks"}</h3>
          <p className="muted">
            Pilih project di panel <a href="#konteks">Konteks</a> untuk melihat perangkatnya.
          </p>
        </div>
      ) : state.devicesError ? (
        <div className="empty">
          <Icon name="alert" className="tone-error" />
          <h3>Perangkat tidak dapat dimuat</h3>
          <p className="muted">{state.devicesError}</p>
          <button type="button" onClick={() => void session.loadDevices()} disabled={state.devicesLoading}>
            Coba lagi
          </button>
        </div>
      ) : !devices ? (
        <p className="inline-status loading" role="status">
          <Icon name="spinner" className="spin" />
          Memuat perangkat…
        </p>
      ) : devices.length === 0 ? (
        <div className="empty">
          <Icon name="box" />
          <h3>Belum ada perangkat di project ini</h3>
          <p className="muted">
            {can("create")
              ? "Tambahkan perangkat pertama, mis. sensor atau gateway yang terpasang di project ini."
              : "Token Anda tidak boleh menambah perangkat di project ini."}
          </p>
          {can("create") && (
            <button type="button" className="primary" onClick={() => setDialog({ kind: "add" })}>
              <Icon name="plus" />
              Tambah perangkat
            </button>
          )}
        </div>
      ) : (
        <div className="devices" role="table" aria-label="Daftar perangkat">
          <div className="devices-head" role="row">
            <span role="columnheader">Nama</span>
            <span role="columnheader">Nomor seri</span>
            <span role="columnheader">Status</span>
            <span role="columnheader">Lokasi</span>
            <span role="columnheader">Dibuat oleh</span>
            <span role="columnheader">Diperbarui</span>
            <span role="columnheader" className="visually-hidden">
              Aksi
            </span>
          </div>
          <div role="rowgroup">
            {devices.map((entry) => (
              <div className="device-row" role="row" key={entry.id}>
                <span role="cell" className="device-name">
                  {entry.name}
                </span>
                <span role="cell" data-label="Nomor seri">
                  <code title={entry.serialNumber}>{entry.serialNumber}</code>
                </span>
                <span role="cell" data-label="Status">
                  <span className={`chip status-${entry.status}`}>{STATUS_LABEL[entry.status]}</span>
                </span>
                <span role="cell" data-label="Lokasi">
                  {entry.location || "-"}
                </span>
                <span role="cell" data-label="Dibuat oleh">
                  <code title={entry.createdBy || undefined}>{shortId(entry.createdBy)}</code>
                </span>
                <span role="cell" data-label="Diperbarui">
                  <time
                    dateTime={entry.updateTime ?? entry.createTime}
                    title={`Dibuat ${absoluteTime(entry.createTime)}\nDiubah ${absoluteTime(entry.updateTime)}`}
                  >
                    {relativeTime(entry.updateTime ?? entry.createTime, now)}
                  </time>
                </span>
                <span role="cell" className="row-actions">
                  <button
                    type="button"
                    className="quiet"
                    data-action="edit"
                    disabled={!can("edit")}
                    title={can("edit") ? undefined : why("edit")}
                    onClick={() => setDialog({ kind: "edit", device: entry })}
                  >
                    <Icon name="pencil" />
                    Ubah<span className="visually-hidden"> {entry.name}</span>
                  </button>
                  <button
                    type="button"
                    className="danger-quiet"
                    data-action="delete"
                    disabled={!can("delete")}
                    title={can("delete") ? undefined : why("delete")}
                    onClick={() => setDialog({ kind: "delete", device: entry })}
                  >
                    <Icon name="trash" />
                    Hapus<span className="visually-hidden"> {entry.name}</span>
                  </button>
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {dialog?.kind === "add" && <DeviceFormDialog onSave={save(undefined)} onClose={() => setDialog(undefined)} />}
      {dialog?.kind === "edit" && (
        <DeviceFormDialog device={dialog.device} onSave={save(dialog.device)} onClose={() => setDialog(undefined)} />
      )}
      {dialog?.kind === "delete" && (
        <DeleteDialog device={dialog.device} onConfirm={remove(dialog.device)} onClose={() => setDialog(undefined)} />
      )}
    </section>
  );
}
