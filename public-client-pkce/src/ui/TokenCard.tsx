import { type Claims, decodeClaims } from "../jwt";
import { CopyButton } from "./CopyButton";
import { clock, remaining } from "./time";

export interface TokenTimes {
  issuedAt: number;
  expiresAt: number;
  // When the SPA stops reusing the token and fetches a new one.
  renewAt?: number;
}

type Status = "active" | "soon" | "expired";

const LABEL: Record<Status, string> = { active: "Aktif", soon: "Segera diperbarui", expired: "Kedaluwarsa" };

function statusOf(now: number, times: TokenTimes): Status {
  if (now >= times.expiresAt) return "expired";
  if (times.renewAt !== undefined && now >= times.renewAt) return "soon";
  return "active";
}

function asList(value: unknown): string[] {
  if (typeof value === "string") return value ? value.split(" ") : [];
  return Array.isArray(value) ? value.filter((v): v is string => typeof v === "string") : [];
}

export function TokenCard(props: { title: string; note: string; token: string; now: number; times?: TokenTimes }) {
  const claims = decodeClaims(props.token);
  const times = props.times ?? timesFromClaims(claims);
  const status = times ? statusOf(props.now, times) : undefined;
  const left = times ? Math.max(0, times.expiresAt - props.now) : 0;
  const life = times ? Math.max(1, times.expiresAt - times.issuedAt) : 1;
  const percent = Math.round(Math.min(100, (left / life) * 100));
  const claimsJson = claims ? JSON.stringify(claims, null, 2) : undefined;

  return (
    <article className="card token">
      <header className="card-head">
        <h3>{props.title}</h3>
        {status && <span className={`badge badge-${status}`}>{LABEL[status]}</span>}
      </header>
      <p className="muted small">{props.note}</p>

      {times && (
        <div className="life">
          <div className="life-row">
            <span>{status === "expired" ? "Sudah kedaluwarsa" : `Sisa ${remaining(left)}`}</span>
            <span className="muted small">dari {remaining(life)}</span>
          </div>
          <div
            className={`bar bar-${status}`}
            role="progressbar"
            aria-label={`Sisa umur ${props.title}`}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={percent}
          >
            <span style={{ width: `${percent}%` }} />
          </div>
          <dl className="times">
            <div>
              <dt>Terbit</dt>
              <dd>{clock(times.issuedAt)}</dd>
            </div>
            <div>
              <dt>Kedaluwarsa</dt>
              <dd>{clock(times.expiresAt)}</dd>
            </div>
            {times.renewAt !== undefined && (
              <div>
                <dt>Pembaruan</dt>
                <dd>{clock(times.renewAt)}</dd>
              </div>
            )}
          </dl>
        </div>
      )}

      {claims ? <KeyClaims claims={claims} /> : <p className="muted small">Token ini bukan JWT.</p>}

      <div className="token-actions">
        {claimsJson && (
          <details className="claims">
            <summary>Lihat klaim lengkap</summary>
            <pre tabIndex={0}>{claimsJson}</pre>
            <CopyButton value={claimsJson} label="Salin klaim" />
          </details>
        )}
        <CopyButton value={props.token} label="Salin token" />
      </div>
    </article>
  );
}

function KeyClaims({ claims }: { claims: Claims }) {
  const aud = asList(claims.aud);
  const perms = asList(claims.perms);
  const prj = typeof claims.prj === "string" ? claims.prj : undefined;
  const org = typeof claims.nice_org_id === "string" ? claims.nice_org_id : undefined;
  if (aud.length === 0 && !prj && !org && perms.length === 0) return null;
  return (
    <dl className="key-claims">
      {aud.length > 0 && (
        <>
          <dt>aud</dt>
          <dd>
            {aud.map((value) => (
              <code key={value} className="chip">
                {value}
              </code>
            ))}
          </dd>
        </>
      )}
      {prj && (
        <>
          <dt>prj</dt>
          <dd>
            <code className="chip">{prj}</code>
          </dd>
        </>
      )}
      {org && (
        <>
          <dt>nice_org_id</dt>
          <dd>
            <code className="chip">{org}</code>
          </dd>
        </>
      )}
      {perms.length > 0 && (
        <>
          <dt>perms</dt>
          <dd>
            {perms.map((value) => (
              <code key={value} className="chip">
                {value}
              </code>
            ))}
          </dd>
        </>
      )}
    </dl>
  );
}

function timesFromClaims(claims: Claims | undefined): TokenTimes | undefined {
  if (typeof claims?.iat !== "number" || typeof claims.exp !== "number") return undefined;
  return { issuedAt: claims.iat * 1000, expiresAt: claims.exp * 1000 };
}
