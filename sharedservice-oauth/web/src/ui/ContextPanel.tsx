import { refusalReason, type Session, type SessionState } from "../session";
import { ContextPicker } from "./ContextPicker";
import { Icon } from "./icons";

export function ContextPanel({ session, state }: { session: Session; state: SessionState }) {
  const busy = state.switching !== undefined;
  const contexts = state.contexts;

  return (
    <section id="konteks" className="card" aria-labelledby="konteks-title">
      <header className="card-head">
        <h2 id="konteks-title">Konteks</h2>
        <button type="button" className="quiet" disabled={state.contextsLoading} onClick={() => void session.reloadContexts()}>
          <Icon name="refresh" />
          Muat ulang
        </button>
      </header>

      <div aria-live="polite">
        {state.switching && (
          <p className="inline-status">
            <Icon name="spinner" className="spin" />
            Meminta backend pindah ke konteks baru…
          </p>
        )}
        {state.switchError && (
          <p className="callout warning" role="alert">
            <Icon name="alert" />
            <span>{state.switchError}</span>
          </p>
        )}
        {state.contextError && (
          <p className="callout warning" role="alert" data-context-error={state.contextError}>
            <Icon name="alert" />
            <span>Token untuk konteks aktif tidak dapat diperoleh. {refusalReason(state.contextError)} Pilih konteks lain.</span>
          </p>
        )}
        {state.contextsError && (
          <p className="callout warning" role="alert">
            <Icon name="alert" />
            <span>{state.contextsError}</span>
          </p>
        )}
      </div>

      {contexts ? (
        <>
          <p className="muted small">
            {state.active ? "" : "Pilih project untuk melihat barangnya. "}
            Pilihan berasal dari <code>GET /auth/contexts</code>, yang dibaca backend dari NICE. Memilih project meminta backend
            menjalankan satu refresh token, tanpa meninggalkan halaman ini.
          </p>
          {contexts.organizations.length === 0 ? (
            <p className="muted small">NICE tidak mengembalikan organisasi apa pun untuk akun ini.</p>
          ) : (
            <ContextPicker
              contexts={contexts}
              active={state.active}
              busy={busy}
              onChoose={(org, project) => void session.choose(org, project)}
            />
          )}
        </>
      ) : state.contextsError ? null : (
        <p className="inline-status">
          <Icon name="spinner" className="spin" />
          Memuat konteks…
        </p>
      )}
    </section>
  );
}
