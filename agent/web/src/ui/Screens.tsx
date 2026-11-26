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

export function SignIn(props: { signedOut?: boolean; onSignIn: () => void; onSelectAccount: () => void }) {
  return (
    <ScreenCard icon="chat" title="Agen">
      {props.signedOut ? (
        <p role="status">
          Anda sudah keluar dari agen. Sesi NICE di browser ini tetap ada, jadi Masuk bisa langsung kembali tanpa kata
          sandi.
        </p>
      ) : (
        <p>Masuk lewat NICE untuk meminta agen memanggil service di organisasi dan project Anda.</p>
      )}
      <button type="button" className="primary block" onClick={props.onSignIn}>
        Masuk
      </button>
      <button type="button" className="quiet block" onClick={props.onSelectAccount}>
        Masuk dengan akun lain
      </button>
      <details className="plain-details">
        <summary>Cara kerjanya</summary>
        <p className="muted">
          Halaman ini tidak memegang token dan tidak memanggil NICE, provider LLM, atau service. Backend agen masuk ke NICE
          sebagai service client confidential, menyimpan refresh token di sisi server, dan memberi browser cookie sesi.
          Backend juga yang menjalankan model LLM dan memanggil service atas nama Anda.
        </p>
      </details>
    </ScreenCard>
  );
}
