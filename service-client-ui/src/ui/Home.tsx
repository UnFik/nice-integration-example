import { decodeClaims } from "../jwt";
import type { Session, SessionState } from "../session";
import type { TokenView } from "../token-manager";
import { ContextPicker } from "./ContextPicker";

export function Home({ session, state }: { session: Session; state: SessionState }) {
  const { config } = session;
  const userinfo = state.identity?.userinfo;
  const name = typeof userinfo?.name === "string" && userinfo.name ? userinfo.name : undefined;
  const active = state.active;
  const org = state.contexts?.organizations.find((candidate) => candidate.id === active?.org);
  const project = org?.projects.find((candidate) => candidate.id === active?.project);

  return (
    <main className="page">
      <header className="topbar">
        <div>
          <h1>Inventaris</h1>
          <p className="muted">
            Masuk sebagai {name ?? "user"} · service client <code>{config.clientId}</code>
          </p>
        </div>
        <button type="button" className="secondary" onClick={() => void session.logout()}>
          Keluar
        </button>
      </header>

      <section>
        <h2>Identitas</h2>
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
              <span className="muted"> (organisasi pilihan NICE saat login, dari klaim access token client)</span>
            </dd>
          </dl>
        ) : (
          <p className={state.identityError ? "error-text" : "muted"}>{state.identityError ?? "Memuat userinfo…"}</p>
        )}
      </section>

      <section>
        <header className="card-head">
          <h2>Konteks</h2>
          <button
            type="button"
            className="secondary"
            disabled={state.contextsLoading}
            onClick={() => void session.reloadContexts()}
          >
            Muat ulang
          </button>
        </header>
        <p className="muted">
          Pilihan di bawah hanya berasal dari <code>GET /api/v1/me/contexts</code>. Memilih project menjalankan satu
          refresh token, tanpa meninggalkan halaman ini.
        </p>
        {state.switching && <p className="muted">Meminta token untuk konteks baru…</p>}
        {state.switchError && <p className="error-text">{state.switchError}</p>}
        {state.contextsError && <p className="error-text">{state.contextsError}</p>}
        {state.contexts && active ? (
          <ContextPicker
            contexts={state.contexts}
            active={active}
            busy={state.switching !== undefined}
            onChoose={(orgId, projectId) => void session.choose(orgId, projectId)}
          />
        ) : (
          !state.contextsError && <p className="muted">Memuat konteks…</p>
        )}
      </section>

      <section>
        <header className="card-head">
          <h2>Barang{project && org ? ` di ${org.name} / ${project.name}` : ""}</h2>
          {active?.project && (
            <button type="button" className="secondary" disabled={state.itemsLoading} onClick={() => void session.loadItems()}>
              Muat ulang
            </button>
          )}
        </header>
        {!active?.project ? (
          <p className="muted">Pilih project di atas untuk melihat barangnya.</p>
        ) : state.itemsError ? (
          <p className="error-text">{state.itemsError}</p>
        ) : state.items ? (
          state.items.length === 0 ? (
            <p className="muted">Belum ada barang di organisasi ini.</p>
          ) : (
            <table className="items">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Nama</th>
                  <th>Jumlah</th>
                </tr>
              </thead>
              <tbody>
                {state.items.map((entry) => (
                  <tr key={entry.id}>
                    <td>
                      <code>{entry.id}</code>
                    </td>
                    <td>{entry.name}</td>
                    <td>{entry.quantity}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )
        ) : (
          <p className="muted">Memuat barang…</p>
        )}
      </section>

      <section>
        <h2>Token yang dipegang UI</h2>
        <p className="warning">
          Klaim di bawah didekode di browser dan <strong>tidak diverifikasi oleh UI</strong>. Hanya di memori, satu per
          (organisasi, service, project); sessionStorage hanya menyimpan refresh token dan ID token.
        </p>
        <div className="grid">
          {state.tokens.map((held) => (
            <TokenCard key={held.key} held={held} />
          ))}
        </div>
      </section>
    </main>
  );
}

function TokenCard({ held }: { held: TokenView }) {
  const claims = decodeClaims(held.access);
  const title = !held.service
    ? "Access token client"
    : held.service === "nice"
      ? "Token API NICE terdelegasi"
      : `Token ${held.service}`;
  return (
    <article className="card">
      <h3>{title}</h3>
      <p className="muted">
        <code>{held.org}</code>
        {held.project && (
          <>
            {" "}
            · <code>{held.project}</code>
          </>
        )}
      </p>
      <p className="muted">Dipakai ulang sampai {time(Math.min(held.refreshAt, held.expiresAt - 60_000))}.</p>
      <pre>{claims ? JSON.stringify(claims, null, 2) : "Token ini bukan JWT."}</pre>
    </article>
  );
}

function text(value: unknown): string {
  return typeof value === "string" && value ? value : "-";
}

function time(ms: number): string {
  return new Date(ms).toLocaleTimeString("id-ID");
}
