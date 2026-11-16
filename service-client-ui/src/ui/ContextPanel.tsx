import type { Session, SessionState } from "../session";
import { ContextPicker } from "./ContextPicker";
import { ManualContextForm } from "./ManualContextForm";
import { Icon } from "./icons";

export function ContextPanel({ session, state }: { session: Session; state: SessionState }) {
  const active = state.active;
  const busy = state.switching !== undefined;
  const discovered = state.contexts && active;
  const unavailable = !discovered && state.contextsError !== undefined;

  return (
    <section id="konteks" className="card" aria-labelledby="konteks-title">
      <header className="card-head">
        <h2 id="konteks-title">Konteks</h2>
        <button
          type="button"
          className="quiet"
          disabled={state.contextsLoading}
          onClick={() => void session.reloadContexts()}
        >
          <Icon name="refresh" />
          Muat ulang
        </button>
      </header>

      <div aria-live="polite">
        {state.switching && (
          <p className="inline-status">
            <Icon name="spinner" className="spin" />
            Meminta token untuk konteks baru…
          </p>
        )}
        {state.switchError && (
          <p className="callout warning" role="alert">
            <Icon name="alert" />
            <span>{state.switchError}</span>
          </p>
        )}
      </div>

      {discovered ? (
        <>
          <p className="muted small">
            Pilihan berasal dari <code>GET /api/v1/me/contexts</code>. Memilih project menjalankan satu refresh token,
            tanpa meninggalkan halaman ini.
          </p>
          <ContextPicker
            contexts={state.contexts!}
            active={active}
            busy={busy}
            onChoose={(orgId, projectId) => void session.choose(orgId, projectId)}
          />
        </>
      ) : unavailable ? (
        <>
          <p className="callout info">
            <Icon name="info" />
            <span>
              NICE tidak lagi menyediakan penemuan organisasi dan project untuk SPA publik, jadi konteks dipilih dengan
              memasukkan ID-nya.
            </span>
          </p>
          <details className="plain-details">
            <summary>Detail teknis</summary>
            <p className="muted small">{state.contextsError}</p>
          </details>
          <ManualContextForm
            defaultOrg={state.identity?.orgId ?? active?.org}
            recent={state.recent ?? []}
            busy={busy}
            onUse={(orgId, projectId) => session.chooseManual(orgId, projectId)}
          />
        </>
      ) : (
        <p className="inline-status">
          <Icon name="spinner" className="spin" />
          Memuat konteks…
        </p>
      )}
    </section>
  );
}
