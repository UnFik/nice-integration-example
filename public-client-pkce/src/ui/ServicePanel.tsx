import { useEffect, useRef, useState } from "react";
import type { ServiceConfig } from "../config";
import type { Session } from "../session";
import { TokenError } from "../token-manager";

type Outcome =
  | { kind: "response"; status: number; statusText: string; description?: string; body: string }
  | { kind: "token-error"; message: string }
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
    try {
      const response = await session.call(service);
      const body = await response.text();
      setOutcome({
        kind: "response",
        status: response.status,
        statusText: response.statusText,
        description: errorDescription(body),
        body: pretty(body),
      });
    } catch (error) {
      if (error instanceof TokenError) {
        // A session that ended switches the whole app to "Sesi berakhir".
        if (error.kind !== "session_ended") setOutcome({ kind: "token-error", message: error.message });
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
    <article className="card">
      <header className="card-head">
        <h3>{service.slug}</h3>
        <button type="button" onClick={() => void call()} disabled={busy}>
          {busy ? "Memanggil…" : "Panggil"}
        </button>
      </header>
      <p className="muted">
        <code>GET {service.endpoint + service.path}</code>
        {service.project ? (
          <>
            {" "}
            · project <code>{service.project}</code>
          </>
        ) : (
          " · project bawaan OAuth client"
        )}
      </p>
      {outcome?.kind === "response" && (
        <div className="result">
          <p>
            Status HTTP:{" "}
            <strong className={outcome.status < 400 ? "ok-text" : "error-text"}>
              {outcome.status} {outcome.statusText}
            </strong>
          </p>
          {outcome.description && (
            <p>
              <code>error_description</code>: <strong className="error-text">{outcome.description}</strong>
            </p>
          )}
          <pre>{outcome.body || "(body kosong)"}</pre>
        </div>
      )}
      {outcome?.kind === "token-error" && (
        <p className="error-text">Token service tidak diperoleh. {outcome.message}</p>
      )}
      {outcome?.kind === "call-error" && <p className="error-text">Service tidak dapat dipanggil: {outcome.message}</p>}
    </article>
  );
}

function errorDescription(body: string): string | undefined {
  try {
    const value: unknown = JSON.parse(body);
    const description = (value as { error_description?: unknown } | null)?.error_description;
    return typeof description === "string" ? description : undefined;
  } catch {
    return undefined;
  }
}

function pretty(body: string): string {
  try {
    return JSON.stringify(JSON.parse(body), null, 2);
  } catch {
    return body;
  }
}
