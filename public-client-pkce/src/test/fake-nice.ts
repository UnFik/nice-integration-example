export const CLIENT_ID = "cli-spa";
export const REDIRECT_URI = "http://localhost:5180/callback";
export const POST_LOGOUT_URI = "http://localhost:5180/";
export const SUBJECT = "usr_01a17b49-56bc-7632-a32f-d47311c068c1";
export const ORG = "org_01a17b49-56bc-7632-a32f-d47311c068c1";

type Handler = (url: URL, init: RequestInit) => Response | Promise<Response> | undefined;

interface PendingCode {
  challenge: string;
  nonce: string;
  redirectUri: string;
}

export class FakeNice {
  readonly issuer: string;
  refreshToken = "rt-0";
  latencyMs = 0;
  accessTtlSeconds = 1800;
  revokeAnswer: "ok" | "error" | "network" = "ok";
  idTokenNonce?: string;
  issueRefreshTokenOnLogin = true;
  tokenOverride?: (form: URLSearchParams) => Response | Promise<Response> | undefined;
  service?: Handler;

  readonly tokenRequests: URLSearchParams[] = [];
  readonly revokeRequests: URLSearchParams[] = [];
  readonly serviceRequests: { url: string; authorization: string | null }[] = [];
  readonly urls: string[] = [];
  inFlight = 0;
  maxInFlight = 0;

  private rotations = 0;
  private accessCount = 0;
  private readonly codes = new Map<string, PendingCode>();

  constructor(
    readonly now: () => number = Date.now,
    issuer = "https://nice.test",
  ) {
    this.issuer = issuer;
  }

  readonly fetch: typeof fetch = async (input, init = {}) => {
    const url = new URL(input instanceof Request ? input.url : String(input));
    this.urls.push(url.href);
    if (url.origin === new URL(this.issuer).origin) return this.nice(url, init);
    this.serviceRequests.push({ url: url.href, authorization: new Headers(init.headers).get("authorization") });
    const answer = await this.service?.(url, init);
    return answer ?? json(404, { error: "not_found" });
  };

  // authorize stands in for the NICE login page, which the tests cannot drive.
  authorize(authorizationUrl: string): string {
    const params = new URL(authorizationUrl).searchParams;
    const code = `code-${this.codes.size + 1}`;
    this.codes.set(code, {
      challenge: params.get("code_challenge") ?? "",
      nonce: params.get("nonce") ?? "",
      redirectUri: params.get("redirect_uri") ?? "",
    });
    const callback = new URL(params.get("redirect_uri") ?? REDIRECT_URI);
    callback.searchParams.set("code", code);
    callback.searchParams.set("state", params.get("state") ?? "");
    return callback.href;
  }

  accessToken(aud: string, extra: Record<string, unknown> = {}): string {
    const iat = Math.floor(this.now() / 1000);
    this.accessCount += 1;
    return fakeJwt({
      iss: this.issuer,
      sub: SUBJECT,
      aud,
      iat,
      exp: iat + this.accessTtlSeconds,
      jti: `at-${this.accessCount}`,
      nice_org_id: ORG,
      nice_principal_type: "user",
      ...extra,
    });
  }

  idToken(claims: Record<string, unknown> = {}): string {
    const iat = Math.floor(Date.now() / 1000);
    return fakeJwt({ iss: this.issuer, sub: SUBJECT, aud: CLIENT_ID, iat, exp: iat + 3600, auth_time: iat, ...claims });
  }

  rotate(): string {
    this.rotations += 1;
    this.refreshToken = `rt-${this.rotations}`;
    return this.refreshToken;
  }

  private async nice(url: URL, init: RequestInit): Promise<Response> {
    switch (url.pathname) {
      case "/.well-known/openid-configuration":
        return json(200, {
          issuer: this.issuer,
          authorization_endpoint: `${this.issuer}/oauth/authorize`,
          token_endpoint: `${this.issuer}/oauth/token`,
          userinfo_endpoint: `${this.issuer}/oauth/userinfo`,
          revocation_endpoint: `${this.issuer}/oauth/revoke`,
          end_session_endpoint: `${this.issuer}/oauth/end_session`,
          jwks_uri: `${this.issuer}/oauth/keys`,
          code_challenge_methods_supported: ["S256"],
          id_token_signing_alg_values_supported: ["RS256"],
        });
      case "/oauth/token":
        return this.token(form(init));
      case "/oauth/userinfo":
        if (!new Headers(init.headers).get("authorization")?.startsWith("Bearer ")) return json(401, {});
        return json(200, { sub: SUBJECT, name: "Siti Rahma", preferred_username: "siti@nusapala.id" });
      case "/oauth/revoke":
        this.revokeRequests.push(form(init));
        if (this.revokeAnswer === "network") throw new TypeError("Failed to fetch");
        return this.revokeAnswer === "ok" ? new Response(null, { status: 200 }) : json(503, { error: "server_error" });
    }
    return json(404, { error: "not_found" });
  }

  private async token(body: URLSearchParams): Promise<Response> {
    this.tokenRequests.push(body);
    this.inFlight += 1;
    this.maxInFlight = Math.max(this.maxInFlight, this.inFlight);
    try {
      if (this.latencyMs > 0) await new Promise((resolve) => setTimeout(resolve, this.latencyMs));
      const override = await this.tokenOverride?.(body);
      if (override) return override;
      if (body.get("grant_type") === "authorization_code") return this.exchange(body);
      if (body.get("grant_type") === "refresh_token") return this.refresh(body);
      return json(400, { error: "unsupported_grant_type" });
    } finally {
      this.inFlight -= 1;
    }
  }

  private async exchange(body: URLSearchParams): Promise<Response> {
    const pending = this.codes.get(body.get("code") ?? "");
    this.codes.delete(body.get("code") ?? "");
    if (!pending || body.get("client_id") !== CLIENT_ID || body.get("redirect_uri") !== pending.redirectUri) {
      return json(400, { error: "invalid_grant" });
    }
    if ((await s256(body.get("code_verifier") ?? "")) !== pending.challenge) return json(400, { error: "invalid_grant" });
    return json(200, {
      access_token: this.accessToken(CLIENT_ID),
      token_type: "Bearer",
      expires_in: this.accessTtlSeconds,
      id_token: this.idToken({ nonce: this.idTokenNonce ?? pending.nonce }),
      ...(this.issueRefreshTokenOnLogin ? { refresh_token: this.rotate() } : {}),
    });
  }

  private refresh(body: URLSearchParams): Response {
    if (body.get("client_id") !== CLIENT_ID) return json(400, { error: "invalid_client" });
    if (body.get("refresh_token") !== this.refreshToken) return json(400, { error: "invalid_grant" });
    const scope = body.get("scope") ?? "";
    const aud = /nice:aud:(\S+)/.exec(scope)?.[1] ?? CLIENT_ID;
    const prj = /nice:prj:(\S+)/.exec(scope)?.[1];
    return json(200, {
      access_token: this.accessToken(aud, { scope, ...(prj ? { prj } : {}) }),
      token_type: "Bearer",
      expires_in: this.accessTtlSeconds,
      refresh_token: this.rotate(),
      scope,
    });
  }
}

export function json(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json", ...headers },
  });
}

export function fakeJwt(payload: Record<string, unknown>): string {
  return `${base64Url(JSON.stringify({ alg: "RS256", typ: "JWT", kid: "test" }))}.${base64Url(JSON.stringify(payload))}.c2lnbmF0dXJl`;
}

function form(init: RequestInit): URLSearchParams {
  return init.body instanceof URLSearchParams ? init.body : new URLSearchParams(String(init.body ?? ""));
}

function base64Url(text: string): string {
  const bytes = new TextEncoder().encode(text);
  return btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}

async function s256(verifier: string): Promise<string> {
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier)));
  return btoa(String.fromCharCode(...digest))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}
