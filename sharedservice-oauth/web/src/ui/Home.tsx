import { type KeyboardEvent, useEffect, useId, useRef, useState } from "react";
import type { Session, SessionState } from "../session";
import { ContextPanel } from "./ContextPanel";
import { initials, shortId } from "./format";
import { Icon } from "./icons";
import { DevicesSection } from "./DevicesSection";
import { ToastRegion, useToasts } from "./Toasts";

export function Home({ session, state }: { session: Session; state: SessionState }) {
  const name = state.user?.name || undefined;
  const active = state.active;
  const org = state.contexts?.organizations.find((candidate) => candidate.id === active?.org);
  const project = org?.projects.find((candidate) => candidate.id === active?.project);
  const orgLabel = active ? (org?.name || shortId(active.org)) : undefined;
  const projectLabel = active ? (project?.name || shortId(active.project)) : undefined;
  const { toasts, notify, dismiss } = useToasts();

  return (
    <div className="app">
      <a className="skip-link" href="#konten">
        Lewati ke konten
      </a>
      <header className="topbar">
        <div className="topbar-inner">
          <div className="brand">
            <Icon name="box" />
            <div>
              <strong>NADI</strong>
              <span className="muted small">Contoh SharedService NICE</span>
            </div>
          </div>
          <ul className="context-chips" aria-label="Konteks aktif">
            <li className={`chip-context${orgLabel ? "" : " is-empty"}`} title={active?.org}>
              <Icon name="org" />
              <span className="visually-hidden">Organisasi: </span>
              {orgLabel ?? "Belum ada"}
            </li>
            <li className={`chip-context${projectLabel ? "" : " is-empty"}`} title={active?.project}>
              <Icon name="project" />
              <span className="visually-hidden">Project: </span>
              {projectLabel ?? "Belum dipilih"}
            </li>
          </ul>
          <UserMenu session={session} state={state} name={name} />
        </div>
      </header>

      <main id="konten" className="wrap">
        <div className="page-head">
          <nav aria-label="Posisi konteks">
            <ol className="breadcrumb">
              <li>
                <span className="muted">Organisasi</span> <strong>{orgLabel ?? "-"}</strong>
              </li>
              <li aria-current={projectLabel ? "page" : undefined}>
                <span className="muted">Project</span> <strong>{projectLabel ?? "belum dipilih"}</strong>
              </li>
            </ol>
          </nav>
          <p className="muted small">
            Masuk sebagai {name ?? "user"}
            {state.user?.email ? ` (${state.user.email})` : ""}
          </p>
        </div>

        {state.logoutError && (
          <div className="callout warning logout-error" role="alert">
            <Icon name="alert" />
            <span>{state.logoutError}</span>
            <button type="button" disabled={state.loggingOut} onClick={() => void session.retrySignOut()}>
              Coba lagi
            </button>
          </div>
        )}

        <div className="layout">
          <div className="stack">
            <DevicesSection session={session} state={state} notify={notify} />
          </div>
          <aside className="stack side" aria-label="Konteks dan identitas">
            <ContextPanel session={session} state={state} />
            <Identity state={state} />
          </aside>
        </div>
      </main>
      <ToastRegion toasts={toasts} onDismiss={dismiss} />
    </div>
  );
}

// UserMenu holds the two ways out: "Ganti akun" ends the session of nadi and
// shows the account chooser of NICE, "Keluar" also ends the NICE session.
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

function Identity({ state }: { state: SessionState }) {
  const user = state.user;
  return (
    <section className="card" aria-labelledby="identitas-title">
      <header className="card-head">
        <h2 id="identitas-title">Identitas</h2>
      </header>
      <dl className="facts">
        <dt>ID user</dt>
        <dd>
          <code>{text(user?.id)}</code>
        </dd>
        <dt>Nama</dt>
        <dd>{text(user?.name)}</dd>
        <dt>Email</dt>
        <dd>{text(user?.email)}</dd>
        <dt>Izin</dt>
        <dd>
          {state.perms.length > 0 ? (
            <ul className="chips" aria-label="Izin token konteks aktif">
              {state.perms.map((perm) => (
                <li key={perm}>
                  <code>{perm}</code>
                </li>
              ))}
            </ul>
          ) : (
            <span className="muted">-</span>
          )}
          <span className="muted small"> dari token konteks aktif, dibaca backend dan dikirim lewat /auth/session</span>
        </dd>
      </dl>
    </section>
  );
}

function text(value: string | undefined): string {
  return value || "-";
}
