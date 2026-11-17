import type { Session, SessionState } from "../session";
import { ContextPanel } from "./ContextPanel";
import { initials, shortId } from "./format";
import { Icon } from "./icons";
import { ItemsSection } from "./ItemsSection";
import { ToastRegion, useToasts } from "./Toasts";
import { TokensSection } from "./TokensSection";

export function Home({ session, state }: { session: Session; state: SessionState }) {
  const { config } = session;
  const userinfo = state.identity?.userinfo;
  const name = typeof userinfo?.name === "string" && userinfo.name ? userinfo.name : undefined;
  const active = state.active;
  const org = state.contexts?.organizations.find((candidate) => candidate.id === active?.org);
  const project = org?.projects.find((candidate) => candidate.id === active?.project);
  const orgLabel = active ? (org?.name ?? shortId(active.org)) : undefined;
  const projectLabel = active?.project ? (project?.name ?? shortId(active.project)) : undefined;
  const { toasts, notify, dismiss } = useToasts();
  const activeKey = active?.project ? `${active.org}|inventaris|${active.project}` : undefined;

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
              <strong>Inventaris</strong>
              <span className="muted small">Contoh service client NICE</span>
            </div>
          </div>
          <ul className="context-chips" aria-label="Konteks aktif">
            <li className="chip-context" title={active?.org}>
              <Icon name="org" />
              <span className="visually-hidden">Organisasi: </span>
              {orgLabel ?? "Belum ada"}
            </li>
            <li className={`chip-context${projectLabel ? "" : " empty"}`} title={active?.project}>
              <Icon name="project" />
              <span className="visually-hidden">Project: </span>
              {projectLabel ?? "Belum dipilih"}
            </li>
          </ul>
          <div className="user">
            <span className="avatar" aria-hidden="true">
              {initials(name)}
            </span>
            <span className="user-name">{name ?? "user"}</span>
            <button type="button" onClick={() => void session.logout()}>
              <Icon name="logout" />
              Keluar
            </button>
          </div>
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
            Masuk sebagai {name ?? "user"} · service client <code>{config.clientId}</code>
          </p>
        </div>

        <div className="layout">
          <div className="stack">
            <ItemsSection session={session} state={state} notify={notify} />
            <TokensSection tokens={state.tokens} activeKey={activeKey} />
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

function Identity({ state }: { state: SessionState }) {
  const userinfo = state.identity?.userinfo;
  return (
    <section className="card" aria-labelledby="identitas-title">
      <header className="card-head">
        <h2 id="identitas-title">Identitas</h2>
      </header>
      {userinfo ? (
        <dl className="facts">
          <dt>sub</dt>
          <dd>
            <code>{text(userinfo.sub)}</code>
          </dd>
          <dt>Nama</dt>
          <dd>{text(userinfo.name)}</dd>
          <dt>Email</dt>
          <dd>{text(userinfo.email)}</dd>
          <dt>nice_org_id</dt>
          <dd>
            <code>{text(state.identity?.orgId)}</code>
            <span className="muted small"> organisasi pilihan NICE saat login, dari klaim access token client</span>
          </dd>
        </dl>
      ) : state.identityError ? (
        <p className="callout error" role="alert">
          <Icon name="alert" />
          <span>{state.identityError}</span>
        </p>
      ) : (
        <p className="inline-status">
          <Icon name="spinner" className="spin" />
          Memuat userinfo…
        </p>
      )}
    </section>
  );
}

function text(value: unknown): string {
  return typeof value === "string" && value ? value : "-";
}
