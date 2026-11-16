import type { ReactNode } from "react";
import { Icon, type IconName } from "./icons";

type Tone = "neutral" | "warning" | "error";

function ScreenCard(props: { icon: IconName; tone?: Tone; title: string; children?: ReactNode; wide?: boolean; busy?: boolean }) {
  return (
    <main className="screen">
      <section className={`screen-card${props.wide ? " wide" : ""}`} aria-busy={props.busy || undefined}>
        <span className={`screen-icon ${props.tone ?? "neutral"}${props.busy ? " spin" : ""}`}>
          <Icon name={props.icon} />
        </span>
        <h1>{props.title}</h1>
        {props.children}
      </section>
    </main>
  );
}

export function Notice({ title }: { title: string }) {
  return (
    <ScreenCard icon="spinner" title={title} busy>
      <p className="muted" role="status">
        Mohon tunggu sebentar.
      </p>
    </ScreenCard>
  );
}

export function Problem(props: {
  title: string;
  message?: string;
  action: string;
  onAction: () => void;
  icon?: IconName;
  tone?: Tone;
}) {
  return (
    <ScreenCard icon={props.icon ?? "alert"} tone={props.tone ?? "error"} title={props.title}>
      {props.message && <p className="screen-message">{props.message}</p>}
      <button type="button" className="primary block" onClick={props.onAction}>
        {props.action}
      </button>
    </ScreenCard>
  );
}

export function SignIn(props: { issuer: string; clientId: string; onSignIn: () => void }) {
  return (
    <ScreenCard icon="box" title="Inventaris">
      <p>Masuk lewat NICE untuk mengelola barang di organisasi dan project Anda.</p>
      <button type="button" className="primary block" onClick={props.onSignIn}>
        Masuk
      </button>
      <details className="plain-details">
        <summary>Detail koneksi</summary>
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
        <p className="muted">
          UI ini masuk sebagai service client dengan authorization code + PKCE, lalu berpindah organisasi dan project lewat
          refresh token tanpa login ulang.
        </p>
      </details>
    </ScreenCard>
  );
}

export function ConfigScreen(props: { title: string; children: ReactNode }) {
  return (
    <ScreenCard icon="gear" tone="warning" title={props.title} wide>
      {props.children}
    </ScreenCard>
  );
}
