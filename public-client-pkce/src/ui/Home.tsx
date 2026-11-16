import { useState } from "react";
import type { Session, SessionState } from "../session";
import type { TokenView } from "../token-manager";
import { Icon } from "./icons";
import { ServicePanel } from "./ServicePanel";
import { TokenCard } from "./TokenCard";
import { useNow } from "./useNow";

export function Home({ session, state }: { session: Session; state: SessionState }) {
  const { config } = session;
  const userinfo = state.identity?.userinfo;
  const orgId = state.identity?.orgId;
  const name = typeof userinfo?.name === "string" && userinfo.name ? userinfo.name : undefined;
  const email = typeof userinfo?.email === "string" && userinfo.email ? userinfo.email : undefined;
  const clientToken = state.tokens.find((held) => !held.service);
  const [round, setRound] = useState(0);
  const now = useNow();

  return (
    <>
      <header className="topbar">
        <div className="topbar-inner">
          <div className="brand">
            <h1>Contoh public client NICE</h1>
            <p>
              client_id <code>{config.clientId}</code>
            </p>
          </div>
          <div className="who">
            {orgId && (
              <span className="chip chip-org" title={orgId}>
                org <code>{shortId(orgId)}</code>
              </span>
            )}
            <span className="avatar" title={name ?? email ?? "user"} aria-hidden="true">
              {initials(name ?? email)}
            </span>
            <button type="button" className="btn btn-quiet" onClick={() => void session.logout()}>
              Keluar
            </button>
          </div>
        </div>
      </header>

      <main className="page">
        <div className="layout">
          <section className="card identity" aria-labelledby="h-identitas">
            <h2 id="h-identitas">Identitas</h2>
            {userinfo ? (
              <>
                <div className="who-line">
                  <span className="avatar avatar-lg" aria-hidden="true">
                    {initials(name ?? email)}
                  </span>
                  <div>
                    <p className="who-name">Masuk sebagai {name ?? "user"}</p>
                    <p className="muted small">dari /oauth/userinfo</p>
                  </div>
                </div>
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
                    <code>{text(orgId)}</code>
                    <span className="muted small"> (klaim access token client, bukan userinfo)</span>
                  </dd>
                </dl>
                <details className="claims">
                  <summary>
                    Jawaban lengkap <code>/oauth/userinfo</code>
                  </summary>
                  <pre tabIndex={0}>{JSON.stringify(userinfo, null, 2)}</pre>
                </details>
              </>
            ) : state.identityError ? (
              <div className="alert">
                <Icon name="alert" size={18} />
                <div>
                  <strong>Identitas tidak dapat dimuat</strong>
                  <p>{state.identityError}</p>
                </div>
              </div>
            ) : (
              <div className="skeleton-stack" role="status">
                <span className="visually-hidden">Memuat userinfo…</span>
                <span className="skeleton" aria-hidden="true" />
                <span className="skeleton short" aria-hidden="true" />
                <span className="skeleton" aria-hidden="true" />
              </div>
            )}
          </section>

          <section className="services" aria-labelledby="h-service">
            <div className="section-head">
              <div>
                <h2 id="h-service">Service</h2>
                <p className="muted small">Tiap service memakai token terpisah yang diminta saat dipanggil.</p>
              </div>
              {config.services.length > 1 && (
                <button type="button" className="btn btn-secondary" onClick={() => setRound((n) => n + 1)}>
                  Panggil semua
                </button>
              )}
            </div>
            {config.services.length === 0 ? (
              <div className="card empty">
                <p>
                  Belum ada service di <code>VITE_SERVICES</code>.
                </p>
                <p className="muted small">Tambahkan entri di .env, lalu jalankan ulang npm run dev.</p>
              </div>
            ) : (
              <div className="stack">
                {config.services.map((service, index) => (
                  <ServicePanel key={index} service={service} session={session} round={round} />
                ))}
              </div>
            )}
          </section>
        </div>

        <section className="tokens" aria-labelledby="h-token">
          <div className="section-head">
            <div>
              <h2 id="h-token">Token yang dipegang SPA</h2>
              <p className="muted small">Sisa waktu dihitung dari klaim iat dan exp.</p>
            </div>
          </div>
          <p className="notice">
            Klaim di bawah didekode di browser dan <strong>tidak diverifikasi oleh SPA</strong>. Service memverifikasi
            tokennya sendiri.
          </p>
          <div className="grid">
            {clientToken ? (
              <TokenCard
                title="Access token client"
                note="aud berisi client_id SPA; dipakai untuk userinfo. Setelah login ini token dari penukaran kode, setelah reload diambil lewat refresh dengan scope dasar saja. Hanya di memori."
                token={clientToken.access}
                now={now}
                times={timesOf(clientToken)}
              />
            ) : (
              <article className="card token">
                <h3>Access token client</h3>
                <p className="muted small">Sedang diambil lewat refresh token dengan scope dasar.</p>
                <div className="skeleton-stack" aria-hidden="true">
                  <span className="skeleton" />
                  <span className="skeleton short" />
                </div>
              </article>
            )}
            {state.idToken && (
              <TokenCard title="ID token" note="Disimpan di sessionStorage." token={state.idToken} now={now} />
            )}
            {state.tokens.flatMap((held) =>
              held.service
                ? [
                    <TokenCard
                      key={held.key}
                      title={`Token service ${held.service.slug}${held.service.project ? ` (${held.service.project})` : ""}`}
                      note="Hanya di memori."
                      token={held.access}
                      now={now}
                      times={timesOf(held)}
                    />,
                  ]
                : [],
            )}
          </div>
        </section>
      </main>
    </>
  );
}

function timesOf(held: TokenView) {
  return { issuedAt: held.issuedAt, expiresAt: held.expiresAt, renewAt: Math.min(held.refreshAt, held.expiresAt - 60_000) };
}

function text(value: unknown): string {
  return typeof value === "string" && value ? value : "-";
}

function shortId(id: string): string {
  return id.length > 13 ? `${id.slice(0, 8)}…${id.slice(-4)}` : id;
}

function initials(source: string | undefined): string {
  const words = (source ?? "").split(/[\s@._-]+/).filter(Boolean);
  if (words.length === 0) return "?";
  return (words[0][0] + (words.length > 1 ? words[1][0] : "")).toUpperCase();
}
