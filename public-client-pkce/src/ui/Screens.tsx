import type { ReactNode } from "react";
import { Icon, type IconName } from "./icons";

function Stage({ icon, tone = "neutral", children }: { icon: IconName; tone?: "neutral" | "warn" | "error"; children: ReactNode }) {
  return (
    <main className="stage">
      <div className="panel">
        <div className={`panel-icon tone-${tone}`}>
          <Icon name={icon} size={26} className={icon === "spinner" ? "spin" : undefined} />
        </div>
        {children}
      </div>
    </main>
  );
}

export function Notice({ title }: { title: string }) {
  return (
    <Stage icon="spinner">
      <p className="panel-title" role="status">
        {title}
      </p>
      <div className="skeleton-stack" aria-hidden="true">
        <span className="skeleton" />
        <span className="skeleton short" />
      </div>
    </Stage>
  );
}

export function Problem(props: {
  title: string;
  message?: string;
  action: string;
  onAction: () => void;
  tone?: "warn" | "error";
  icon?: IconName;
}) {
  return (
    <Stage icon={props.icon ?? "alert"} tone={props.tone ?? "error"}>
      <h1 className="panel-title">{props.title}</h1>
      {props.message && <p className="panel-text">{props.message}</p>}
      <button type="button" className="btn btn-primary btn-lg" onClick={props.onAction}>
        {props.action}
      </button>
    </Stage>
  );
}

export function SignIn(props: { issuer: string; clientId: string; onSignIn: () => void }) {
  return (
    <Stage icon="lock">
      <h1 className="panel-title">Contoh public client NICE</h1>
      <p className="panel-text">
        SPA ini masuk ke NICE dengan authorization code + PKCE, lalu memanggil service <code>nice_token</code> dengan token
        terpisah per service.
      </p>
      <dl className="facts">
        <dt>Issuer</dt>
        <dd>
          <code>{props.issuer}</code>
        </dd>
        <dt>client_id</dt>
        <dd>
          <code>{props.clientId}</code>
        </dd>
      </dl>
      <button type="button" className="btn btn-primary btn-lg" onClick={props.onSignIn}>
        Masuk
      </button>
      <p className="panel-note">
        Tanpa client secret di browser: tiap login memakai PKCE (S256), jadi kode otorisasi yang tercuri tidak bisa
        ditukar.
      </p>
    </Stage>
  );
}
