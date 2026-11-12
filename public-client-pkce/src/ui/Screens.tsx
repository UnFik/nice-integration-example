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
      <h1>Contoh public client NICE</h1>
      <p>
        SPA ini masuk ke NICE dengan authorization code + PKCE, lalu memanggil service <code>nice_token</code> dengan
        token terpisah per service.
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
