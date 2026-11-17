import { useState } from "react";
import type { Claims } from "../jwt";
import type { TokenView } from "../token-manager";
import { countdown } from "./format";
import { Icon } from "./icons";
import { ageRatio, PHASE_LABEL, remainingMs, tokenPhase } from "./token-status";
import { useNow } from "./useNow";

export function TokensSection({ tokens, activeKey }: { tokens: TokenView[]; activeKey: string | undefined }) {
  const now = useNow(1000);
  const sorted = [...tokens].sort((a, b) => Number(b.key === activeKey) - Number(a.key === activeKey));
  return (
    <section className="card" aria-labelledby="token-title">
      <header className="card-head">
        <h2 id="token-title">Token yang dipegang UI</h2>
      </header>
      <p className="callout info">
        <Icon name="info" />
        <span>
          Klaim didekode di browser dan <strong>tidak diverifikasi oleh UI</strong>. Token hanya di memori, satu per
          (organisasi, service, project). sessionStorage hanya menyimpan refresh token dan ID token, dan keduanya tidak
          pernah ditampilkan.
        </span>
      </p>
      {sorted.length === 0 ? (
        <p className="muted">Belum ada token yang dipegang.</p>
      ) : (
        <div className="token-grid">
          {sorted.map((held) => (
            <TokenCard key={held.key} held={held} now={now} current={held.key === activeKey} />
          ))}
        </div>
      )}
    </section>
  );
}

function TokenCard({ held, now, current }: { held: TokenView; now: number; current: boolean }) {
  const claims = held.claims;
  const phase = tokenPhase(held, now);
  const title = !held.service ? "Access token client" : held.service === "nice" ? "Token API NICE terdelegasi" : `Token ${held.service}`;
  const perms = Array.isArray(claims?.perms) ? claims.perms.filter((p): p is string => typeof p === "string") : [];
  return (
    <article className={`token${current ? " current" : ""}`}>
      <header className="token-head">
        <h3>{title}</h3>
        <span className={`badge ${phase}`}>
          <Icon name={phase === "active" ? "check" : phase === "soon" ? "refresh" : "clock"} />
          {PHASE_LABEL[phase]}
        </span>
      </header>
      <p className="remaining">
        {phase === "expired" ? (
          "Sudah kedaluwarsa"
        ) : (
          <>
            Sisa <strong className="mono">{countdown(remainingMs(held, now))}</strong>
          </>
        )}
      </p>
      <div className={`age ${phase}`} aria-hidden="true">
        <span style={{ width: `${Math.round(ageRatio(held, now) * 100)}%` }} />
      </div>
      <p className="muted small">
        Dipakai ulang sampai {new Date(Math.min(held.refreshAt, held.expiresAt - 60_000)).toLocaleTimeString("id-ID")}.
      </p>
      <dl className="claims">
        <Claim name="aud" value={claims?.aud} />
        <Claim name="prj" value={claims?.prj} />
        <Claim name="nice_org_id" value={claims?.nice_org_id} />
        {perms.length > 0 && (
          <>
            <dt>perms</dt>
            <dd>
              <ul className="chips">
                {perms.map((perm) => (
                  <li key={perm}>
                    <code>{perm}</code>
                  </li>
                ))}
              </ul>
            </dd>
          </>
        )}
      </dl>
      <FullClaims access={held.access} claims={claims} />
    </article>
  );
}

function Claim({ name, value }: { name: string; value: unknown }) {
  if (typeof value !== "string" || !value) return null;
  return (
    <>
      <dt>{name}</dt>
      <dd>
        <code className="chip">{value}</code>
      </dd>
    </>
  );
}

function FullClaims({ access, claims }: { access: string; claims: Claims | undefined }) {
  const [copied, setCopied] = useState<"ok" | "failed">();
  const copy = () => {
    navigator.clipboard.writeText(access).then(
      () => setCopied("ok"),
      () => setCopied("failed"),
    );
  };
  return (
    <details className="plain-details" onToggle={() => setCopied(undefined)}>
      <summary>Lihat klaim lengkap</summary>
      <pre>{claims ? JSON.stringify(claims, null, 2) : "Token ini bukan JWT."}</pre>
      <p className="muted small">JWT mentah (access token)</p>
      <code className="raw">{access}</code>
      <div className="copy-row">
        <button type="button" onClick={copy}>
          <Icon name="copy" />
          Salin JWT
        </button>
        <span role="status" className="small muted">
          {copied === "ok" ? "Tersalin." : copied === "failed" ? "Browser menolak menyalin." : ""}
        </span>
      </div>
    </details>
  );
}
