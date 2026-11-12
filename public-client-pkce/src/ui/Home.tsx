import { useState } from "react";
import { decodeClaims } from "../jwt";
import type { Session, SessionState } from "../session";
import type { TokenView } from "../token-manager";
import { ServicePanel } from "./ServicePanel";

export function Home({ session, state }: { session: Session; state: SessionState }) {
  const { config } = session;
  const userinfo = state.identity?.userinfo;
  const name = typeof userinfo?.name === "string" && userinfo.name ? userinfo.name : undefined;
  const clientToken = state.tokens.find((held) => !held.service);
  const [round, setRound] = useState(0);

  return (
    <main className="page">
      <header className="topbar">
        <div>
          <h1>Contoh public client NICE</h1>
          <p className="muted">
            Masuk sebagai {name ?? "user"} · client_id <code>{config.clientId}</code>
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
              <span className="muted"> (klaim access token client, bukan userinfo)</span>
            </dd>
          </dl>
        ) : (
          <p className={state.identityError ? "error-text" : "muted"}>{state.identityError ?? "Memuat userinfo…"}</p>
        )}
        {userinfo && (
          <details>
            <summary>
              Jawaban lengkap <code>/oauth/userinfo</code>
            </summary>
            <pre>{JSON.stringify(userinfo, null, 2)}</pre>
          </details>
        )}
      </section>

      <section>
        <header className="card-head">
          <h2>Service</h2>
          {config.services.length > 1 && (
            <button type="button" className="secondary" onClick={() => setRound((n) => n + 1)}>
              Panggil semua
            </button>
          )}
        </header>
        {config.services.length === 0 ? (
          <p className="muted">
            Belum ada service di <code>VITE_SERVICES</code>.
          </p>
        ) : (
          <div className="grid">
            {config.services.map((service, index) => (
              <ServicePanel key={index} service={service} session={session} round={round} />
            ))}
          </div>
        )}
      </section>

      <section>
        <h2>Token yang dipegang SPA</h2>
        <p className="warning">
          Klaim di bawah didekode di browser dan <strong>tidak diverifikasi oleh SPA</strong>. Service memverifikasi
          tokennya sendiri.
        </p>
        <div className="grid">
          {clientToken ? (
            <TokenCard
              title="Access token client"
              note={`aud berisi client_id SPA; dipakai untuk userinfo. Setelah login ini token dari penukaran kode, setelah reload diambil lewat refresh dengan scope dasar saja. Hanya di memori. Dipakai ulang sampai ${reuseUntil(clientToken)}.`}
              token={clientToken.access}
            />
          ) : (
            <article className="card">
              <h3>Access token client</h3>
              <p className="muted">Sedang diambil lewat refresh token dengan scope dasar.</p>
            </article>
          )}
          {state.idToken && <TokenCard title="ID token" note="Disimpan di sessionStorage." token={state.idToken} />}
          {state.tokens.flatMap((held) =>
            held.service
              ? [
                  <TokenCard
                    key={held.key}
                    title={`Token service ${held.service.slug}${held.service.project ? ` (${held.service.project})` : ""}`}
                    note={`Hanya di memori. Dipakai ulang sampai ${reuseUntil(held)}.`}
                    token={held.access}
                  />,
                ]
              : [],
          )}
        </div>
      </section>
    </main>
  );
}

function TokenCard({ title, note, token }: { title: string; note: string; token: string }) {
  const claims = decodeClaims(token);
  return (
    <article className="card">
      <h3>{title}</h3>
      <p className="muted">{note}</p>
      {claims && typeof claims.iat === "number" && typeof claims.exp === "number" && (
        <p className="muted">
          Terbit {time(claims.iat * 1000)}, kedaluwarsa {time(claims.exp * 1000)}
        </p>
      )}
      <pre>{claims ? JSON.stringify(claims, null, 2) : "Token ini bukan JWT."}</pre>
    </article>
  );
}

function text(value: unknown): string {
  return typeof value === "string" && value ? value : "-";
}

function reuseUntil(held: TokenView): string {
  return time(Math.min(held.refreshAt, held.expiresAt - 60_000));
}

function time(ms: number): string {
  return new Date(ms).toLocaleTimeString("id-ID");
}
