export function Notice({ title }: { title: string }) {
  return (
    <main className="page narrow">
      <p className="muted">{title}</p>
    </main>
  );
}

export function Problem(props: { title: string; message?: string; action: string; onAction: () => void }) {
  return (
    <main className="page narrow">
      <h1>{props.title}</h1>
      {props.message && <p className="error-text">{props.message}</p>}
      <button type="button" onClick={props.onAction}>
        {props.action}
      </button>
    </main>
  );
}

export function SignIn(props: { issuer: string; clientId: string; onSignIn: () => void }) {
  return (
    <main className="page narrow">
      <h1>Inventaris</h1>
      <p>
        UI service <code>inventaris</code> ini masuk ke NICE sebagai service client dengan authorization code + PKCE, lalu
        berpindah organisasi dan project lewat refresh token, tanpa login ulang.
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
      <button type="button" onClick={props.onSignIn}>
        Masuk
      </button>
    </main>
  );
}
