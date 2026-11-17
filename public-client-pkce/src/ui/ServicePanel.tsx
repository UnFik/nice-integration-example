import { useEffect, useRef, useState } from "react";
import type { ServiceConfig } from "../config";
import type { Session } from "../session";
import { TokenError } from "../token-manager";
import { explainCallError, explainResponse, explainTokenError, type Friendly, parseErrorBody } from "./explain";
import { Icon } from "./icons";

type Outcome =
  | { kind: "response"; status: number; statusText: string; description?: string; code?: string; body: string; ms: number }
  | { kind: "token-error"; message: string; code?: string; temporary: boolean }
  | { kind: "call-error"; message: string };

export function ServicePanel(props: { service: ServiceConfig; session: Session; round: number }) {
  const { service, session, round } = props;
  const [busy, setBusy] = useState(false);
  const [outcome, setOutcome] = useState<Outcome>();
  const lastRound = useRef(round);

  useEffect(() => {
    if (round === lastRound.current) return;
    lastRound.current = round;
    void call();
  });

  async function call() {
    setBusy(true);
    const started = performance.now();
    try {
      const response = await session.call(service);
      const body = await response.text();
      const parsed = parseErrorBody(body);
      setOutcome({
        kind: "response",
        status: response.status,
        statusText: response.statusText,
        description: parsed.description,
        code: parsed.code,
        body: pretty(body),
        ms: Math.round(performance.now() - started),
      });
    } catch (error) {
      if (error instanceof TokenError) {
        // A session that ended switches the whole app to "Sesi berakhir".
        if (error.kind !== "session_ended") {
          setOutcome({ kind: "token-error", message: error.message, code: error.code, temporary: error.kind === "temporary" });
        }
      } else {
        setOutcome({
          kind: "call-error",
          message: `${error instanceof Error ? error.message : String(error)}. Pastikan service berjalan dan CORS-nya mengizinkan origin ${window.location.origin}.`,
        });
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="card service">
      <header className="card-head">
        <h3>{service.slug}</h3>
        <button type="button" className="btn btn-primary" onClick={() => void call()} disabled={busy}>
          {busy && <Icon name="spinner" size={16} className="spin" />}
          {busy ? "Memanggil…" : "Panggil"}
        </button>
      </header>
      <p className="endpoint">
        <code>GET {service.endpoint + service.path}</code>
      </p>
      <p className="muted small">
        {service.project ? (
          <>
            project <code>{service.project}</code>
          </>
        ) : (
          "project bawaan OAuth client"
        )}
      </p>
      <div className="result" aria-live="polite" aria-busy={busy}>
        {!outcome && !busy && <p className="muted small">Belum dipanggil. Hasilnya tampil di sini.</p>}
        {outcome && <Result outcome={outcome} />}
      </div>
    </article>
  );
}

function Result({ outcome }: { outcome: Outcome }) {
  if (outcome.kind === "response") {
    const friendly = explainResponse(outcome.status, { code: outcome.code, description: outcome.description });
    const tone = outcome.status < 300 ? "ok" : outcome.status < 500 ? "warn" : "error";
    return (
      <>
        <div className="result-head">
          <span className={`badge badge-${tone}`}>
            HTTP {outcome.status}
            {outcome.statusText ? ` ${outcome.statusText}` : ""}
          </span>
          <span className="muted small">{outcome.ms} ms</span>
        </div>
        {friendly && <FriendlyError friendly={friendly} />}
        {outcome.description && (
          <p className="small">
            <code>error_description</code>: <strong>{outcome.description}</strong>
          </p>
        )}
        <pre tabIndex={0}>{outcome.body || "(body kosong)"}</pre>
      </>
    );
  }
  if (outcome.kind === "token-error") {
    return (
      <>
        <FriendlyError friendly={explainTokenError(outcome.code, outcome.temporary)} />
        <p className="detail">
          Token service tidak diperoleh. {outcome.code && <code>{outcome.code}</code>} {outcome.message}
        </p>
      </>
    );
  }
  return (
    <>
      <FriendlyError friendly={explainCallError()} />
      <p className="detail">Service tidak dapat dipanggil: {outcome.message}</p>
    </>
  );
}

function FriendlyError({ friendly }: { friendly: Friendly }) {
  return (
    <div className="alert">
      <Icon name="alert" size={18} />
      <div>
        <strong>{friendly.title}</strong>
        <p>{friendly.hint}</p>
      </div>
    </div>
  );
}

function pretty(body: string): string {
  try {
    return JSON.stringify(JSON.parse(body), null, 2);
  } catch {
    return body;
  }
}
