import { type FormEvent, useState } from "react";
import { type Conversation, titleOf } from "../chat";
import type { Session, SessionState } from "../session";
import { relativeTime } from "./format";
import { Icon } from "./icons";

type Mode = { kind: "view" } | { kind: "rename"; id: string; draft: string } | { kind: "delete"; id: string };

export function ConversationList({ session, state }: { session: Session; state: SessionState }) {
  const [mode, setMode] = useState<Mode>({ kind: "view" });
  const [saving, setSaving] = useState(false);
  const usable = state.active !== undefined && !state.contextError;
  const list = state.conversations;

  const rename = async (event: FormEvent, id: string, draft: string) => {
    event.preventDefault();
    setSaving(true);
    const ok = await session.rename(id, draft.trim());
    setSaving(false);
    if (ok) setMode({ kind: "view" });
  };
  const remove = async (id: string) => {
    setSaving(true);
    await session.remove(id);
    setSaving(false);
    setMode({ kind: "view" });
  };

  return (
    <nav className="conversations" aria-labelledby="percakapan-list-title">
      <header className="conversations-head">
        <h2 id="percakapan-list-title">Percakapan</h2>
        <button type="button" className="primary" disabled={!usable || state.turn !== undefined} onClick={() => void session.newConversation()}>
          <Icon name="plus" />
          Baru
        </button>
      </header>

      {!usable ? (
        <p className="muted small">Pilih organisasi dan project di bilah atas untuk membuka percakapan.</p>
      ) : state.conversationsError ? (
        <div className="callout error" role="alert">
          <Icon name="alert" />
          <span>
            {state.conversationsError}{" "}
            <button type="button" className="quiet" onClick={() => void session.loadConversations()}>
              Coba lagi
            </button>
          </span>
        </div>
      ) : !list ? (
        <p className="inline-status">
          <Icon name="spinner" className="spin" />
          Memuat percakapan…
        </p>
      ) : list.length === 0 ? (
        <p className="muted small">Belum ada percakapan. Tulis pesan atau tekan Baru.</p>
      ) : (
        <ul className="conversation-items">
          {list.map((conversation) => (
            <Item
              key={conversation.id}
              conversation={conversation}
              selected={conversation.id === state.selectedId}
              running={state.turn?.conversationId === conversation.id}
              mode={mode}
              saving={saving}
              onSelect={() => void session.select(conversation.id)}
              onMode={setMode}
              onRename={rename}
              onRemove={remove}
            />
          ))}
        </ul>
      )}
    </nav>
  );
}

function Item(props: {
  conversation: Conversation;
  selected: boolean;
  running: boolean;
  mode: Mode;
  saving: boolean;
  onSelect: () => void;
  onMode: (mode: Mode) => void;
  onRename: (event: FormEvent, id: string, draft: string) => void;
  onRemove: (id: string) => void;
}) {
  const { conversation, mode } = props;
  const title = titleOf(conversation);
  const view = () => props.onMode({ kind: "view" });

  if (mode.kind === "rename" && mode.id === conversation.id) {
    return (
      <li className="conversation-item editing">
        <form onSubmit={(event) => props.onRename(event, conversation.id, mode.draft)}>
          <label className="visually-hidden" htmlFor={`judul-${conversation.id}`}>
            Judul percakapan
          </label>
          <input
            id={`judul-${conversation.id}`}
            value={mode.draft}
            maxLength={120}
            autoFocus
            onChange={(event) => props.onMode({ kind: "rename", id: conversation.id, draft: event.target.value })}
            onKeyDown={(event) => {
              if (event.key === "Escape") view();
            }}
          />
          <div className="actions">
            <button type="submit" className="primary" disabled={props.saving || !mode.draft.trim()}>
              Simpan
            </button>
            <button type="button" className="quiet" onClick={view}>
              Batal
            </button>
          </div>
        </form>
      </li>
    );
  }

  if (mode.kind === "delete" && mode.id === conversation.id) {
    return (
      <li className="conversation-item confirming" role="group" aria-label={`Hapus ${title}`}>
        <p className="small">
          Hapus <strong>{title}</strong> beserta semua pesannya?
        </p>
        <div className="actions">
          <button type="button" className="danger" disabled={props.saving} onClick={() => props.onRemove(conversation.id)}>
            Hapus
          </button>
          <button type="button" className="quiet" onClick={view}>
            Batal
          </button>
        </div>
      </li>
    );
  }

  return (
    <li className={`conversation-item${props.selected ? " selected" : ""}`} data-conversation={conversation.id}>
      <button type="button" className="conversation-open" aria-current={props.selected ? "true" : undefined} onClick={props.onSelect}>
        <span className="conversation-title">{title}</span>
        <span className="muted small">
          {props.running ? "Sedang berjalan…" : <time dateTime={conversation.updatedAt}>{relativeTime(conversation.updatedAt, Date.now())}</time>}
        </span>
      </button>
      <div className="conversation-tools">
        <button
          type="button"
          className="icon-button"
          aria-label={`Ganti judul ${title}`}
          onClick={() => props.onMode({ kind: "rename", id: conversation.id, draft: conversation.title })}
        >
          <Icon name="pencil" />
        </button>
        <button
          type="button"
          className="icon-button danger-quiet"
          aria-label={`Hapus ${title}`}
          disabled={props.running}
          onClick={() => props.onMode({ kind: "delete", id: conversation.id })}
        >
          <Icon name="trash" />
        </button>
      </div>
    </li>
  );
}
