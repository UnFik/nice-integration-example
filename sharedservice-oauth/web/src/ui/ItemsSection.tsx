import { useState } from "react";
import { ITEM_PERMISSIONS, type Item, type ItemAction, permissionsOf } from "../items";
import type { Session, SessionState } from "../session";
import { absoluteTime, relativeTime, shortId } from "./format";
import { Icon } from "./icons";
import { DeleteDialog, ItemFormDialog } from "./ItemDialogs";
import { useNow } from "./useNow";

const ACTION_LABEL = { create: "Tambah", edit: "Ubah", delete: "Hapus" } as const;

type Dialog = { kind: "add" } | { kind: "edit"; item: Item } | { kind: "delete"; item: Item };

export function ItemsSection(props: {
  session: Session;
  state: SessionState;
  notify: (kind: "success" | "error", text: string) => void;
}) {
  const { session, state, notify } = props;
  const [dialog, setDialog] = useState<Dialog>();
  const now = useNow(30_000);
  const active = state.active;
  const project = active?.project;

  const held = state.tokens.find(
    (token) => token.service === "inventaris" && token.org === active?.org && token.project === project,
  );
  const perms = permissionsOf(held?.claims);
  const can = (action: ItemAction): boolean => perms?.includes(ITEM_PERMISSIONS[action]) ?? false;
  const missing = (["create", "edit", "delete"] as const).filter((action) => !can(action));
  const why = (action: ItemAction) =>
    perms ? `Token project ini tidak memuat ${ITEM_PERMISSIONS[action]}.` : "Izin di token belum diketahui.";

  const save = (item: Item | undefined) => async (input: { name: string; quantity: number }) => {
    const result = await session.saveItem(input, item?.id);
    if (!result.ok) return result.message;
    setDialog(undefined);
    notify("success", item ? `Barang "${input.name}" diperbarui.` : `Barang "${input.name}" ditambahkan.`);
    return undefined;
  };

  const remove = (item: Item) => async () => {
    const result = await session.removeItem(item.id);
    if (!result.ok) return result.message;
    setDialog(undefined);
    notify("success", `Barang "${item.name}" dihapus.`);
    return undefined;
  };

  const items = state.items;
  return (
    <section className="card" aria-labelledby="barang-title" aria-busy={state.itemsLoading || undefined}>
      <header className="card-head">
        <div>
          <h2 id="barang-title">Barang</h2>
          {project && items && (
            <p className="muted small" aria-live="polite">
              {items.length} barang
            </p>
          )}
        </div>
        {project && (
          <div className="actions">
            <button type="button" className="quiet" disabled={state.itemsLoading} onClick={() => void session.loadItems()}>
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
              Tambah barang
            </button>
          </div>
        )}
      </header>

      {project && items && missing.length > 0 && (
        <p className="callout info perm-note">
          <Icon name="info" />
          <span>
            {perms
              ? `${missing.map((action) => ACTION_LABEL[action]).join(", ")} dinonaktifkan: token project ini tidak memuat ${missing.map((action) => ITEM_PERMISSIONS[action]).join(", ")}.`
              : "Tambah, Ubah, dan Hapus ditahan: izin di token belum diketahui."}
          </span>
        </p>
      )}

      {!project ? (
        <div className="empty">
          <Icon name="box" />
          <h3>Belum ada konteks</h3>
          <p className="muted">
            Pilih project di panel <a href="#konteks">Konteks</a> untuk melihat barangnya.
          </p>
        </div>
      ) : state.itemsError ? (
        <div className="empty">
          <Icon name="alert" className="tone-error" />
          <h3>Barang tidak dapat dimuat</h3>
          <p className="muted">{state.itemsError}</p>
          <button type="button" onClick={() => void session.loadItems()} disabled={state.itemsLoading}>
            Coba lagi
          </button>
        </div>
      ) : !items ? (
        <p className="inline-status loading" role="status">
          <Icon name="spinner" className="spin" />
          Memuat barang dari inventaris…
        </p>
      ) : items.length === 0 ? (
        <div className="empty">
          <Icon name="box" />
          <h3>Belum ada barang di project ini</h3>
          <p className="muted">
            {can("create")
              ? "Tambahkan barang pertama, mis. kabel atau perangkat yang perlu dihitung."
              : "Token Anda tidak boleh menambah barang di project ini."}
          </p>
          {can("create") && (
            <button type="button" className="primary" onClick={() => setDialog({ kind: "add" })}>
              <Icon name="plus" />
              Tambah barang
            </button>
          )}
        </div>
      ) : (
        <div className="items" role="table" aria-label="Daftar barang">
          <div className="items-head" role="row">
            <span role="columnheader">Nama</span>
            <span role="columnheader" className="num">
              Jumlah
            </span>
            <span role="columnheader">Dibuat oleh</span>
            <span role="columnheader">Diperbarui</span>
            <span role="columnheader" className="visually-hidden">
              Aksi
            </span>
          </div>
          <div role="rowgroup">
            {items.map((entry) => (
              <div className="item-row" role="row" key={entry.id}>
                <span role="cell" className="item-name">
                  {entry.name}
                </span>
                <span role="cell" className="num" data-label="Jumlah">
                  {entry.quantity}
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
                    onClick={() => setDialog({ kind: "edit", item: entry })}
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
                    onClick={() => setDialog({ kind: "delete", item: entry })}
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

      {dialog?.kind === "add" && <ItemFormDialog onSave={save(undefined)} onClose={() => setDialog(undefined)} />}
      {dialog?.kind === "edit" && <ItemFormDialog item={dialog.item} onSave={save(dialog.item)} onClose={() => setDialog(undefined)} />}
      {dialog?.kind === "delete" && <DeleteDialog item={dialog.item} onConfirm={remove(dialog.item)} onClose={() => setDialog(undefined)} />}
    </section>
  );
}
