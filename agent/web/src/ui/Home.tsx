import { type KeyboardEvent, useEffect, useId, useRef, useState } from "react";
import { refusalReason, type Session, type SessionState } from "../session";
import { ChatPanel } from "./ChatPanel";
import { ContextSelect } from "./ContextSelect";
import { ConversationList } from "./ConversationList";
import { initials } from "./format";
import { Icon } from "./icons";

export function Home({ session, state }: { session: Session; state: SessionState }) {
  const name = state.user?.name || undefined;

  return (
    <div className="app chat-app">
      <a className="skip-link" href="#percakapan">
        Lewati ke percakapan
      </a>
      <header className="topbar">
        <div className="topbar-inner">
          <div className="brand">
            <Icon name="chat" />
            <div>
              <strong>Agen</strong>
              <span className="muted small">Contoh chat agen NICE</span>
            </div>
          </div>
          <ContextSelect session={session} state={state} />
          <UserMenu session={session} state={state} name={name} />
        </div>
      </header>

      <Notices session={session} state={state} />

      <div className="chat-layout">
        <ConversationList session={session} state={state} />
        <ChatPanel session={session} state={state} />
      </div>
    </div>
  );
}

function Notices({ session, state }: { session: Session; state: SessionState }) {
  const items = [
    state.logoutError && (
      <div key="logout" className="callout warning logout-error" role="alert">
        <Icon name="alert" />
        <span>{state.logoutError}</span>
        <button type="button" disabled={state.loggingOut} onClick={() => void session.retrySignOut()}>
          Coba lagi
        </button>
      </div>
    ),
    state.switchError && (
      <p key="switch" className="callout warning" role="alert">
        <Icon name="alert" />
        <span>{state.switchError}</span>
      </p>
    ),
    state.contextError && (
      <p key="context" className="callout warning" role="alert" data-context-error={state.contextError}>
        <Icon name="alert" />
        <span>Token untuk konteks aktif tidak dapat diperoleh. {refusalReason(state.contextError)} Pilih konteks lain.</span>
      </p>
    ),
    state.contextsError && (
      <p key="contexts" className="callout warning" role="alert">
        <Icon name="alert" />
        <span>{state.contextsError}</span>
      </p>
    ),
  ].filter(Boolean);
  if (items.length === 0) return null;
  return <div className="notices">{items}</div>;
}

// UserMenu holds the two ways out: "Ganti akun" ends the session of the agent
// and shows the account chooser of NICE, "Keluar" also ends the NICE session.
function UserMenu({ session, state, name }: { session: Session; state: SessionState; name: string | undefined }) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const menuId = useId();

  useEffect(() => {
    if (!open) return;
    const outside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    const escape = (event: globalThis.KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setOpen(false);
      trigger.current?.focus();
    };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", escape);
    root.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("keydown", escape);
    };
  }, [open]);

  const choose = (action: () => Promise<void>) => {
    setOpen(false);
    void action();
  };
  const move = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    event.preventDefault();
    const items = [...(root.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])];
    const at = items.indexOf(document.activeElement as HTMLElement);
    const next = (at + (event.key === "ArrowDown" ? 1 : items.length - 1)) % items.length;
    items[next]?.focus();
  };

  return (
    <div className="user" ref={root}>
      <button
        type="button"
        ref={trigger}
        className="user-trigger"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        disabled={state.loggingOut}
        onClick={() => setOpen((value) => !value)}
      >
        <span className="avatar" aria-hidden="true">
          {initials(name)}
        </span>
        <span className="user-name">{state.loggingOut ? "Keluar…" : (name ?? "user")}</span>
        <Icon name={state.loggingOut ? "spinner" : "chevron"} className={state.loggingOut ? "spin" : undefined} />
        <span className="visually-hidden">Menu akun</span>
      </button>
      {open && (
        <div className="user-menu" id={menuId} role="menu" aria-label="Akun" onKeyDown={move}>
          <div className="user-menu-head">
            <strong>{name ?? "user"}</strong>
            {state.user?.email && <span className="muted small">{state.user.email}</span>}
          </div>
          <button type="button" role="menuitem" onClick={() => choose(() => session.switchAccount())}>
            <Icon name="switch" />
            Ganti akun
          </button>
          <button type="button" role="menuitem" onClick={() => choose(() => session.logout())}>
            <Icon name="logout" />
            Keluar
          </button>
        </div>
      )}
    </div>
  );
}
