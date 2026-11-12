export const CLIENT_ID = "cli-inventaris-ui";
export const REDIRECT_URI = "http://localhost:5181/callback";
export const POST_LOGOUT_URI = "http://localhost:5181/";
export const INVENTARIS_URL = "http://127.0.0.1:8081";
export const SUBJECT = "usr_01a17b49-56bc-7632-a32f-d47311c068c1";
export const EMAIL = "siti@nusapala.id";
export const SERVICE = "inventaris";
export const CONTEXTS_PERMISSION = "nice.me.contexts.read";
// LOGIN_SCOPES are the grant scopes of a session the tests start without a
// code exchange, as if the user had signed in with the UI's base scope.
export const LOGIN_SCOPES = ["openid", "profile", "email", "offline_access"];

export const ORG_A = "org_0000000a-0000-7000-8000-000000000000";
export const ORG_B = "org_0000000b-0000-7000-8000-000000000000";
export const ORG_C = "org_0000000c-0000-7000-8000-000000000000";
export const PRJ_P = "prj_0000000f-0000-7000-8000-000000000001";
export const PRJ_Q = "prj_0000000f-0000-7000-8000-000000000002";
export const PRJ_R = "prj_0000000f-0000-7000-8000-000000000003";

export interface FakeProject {
  id: string;
  name: string;
  slug: string;
  // services lists the services active in the project of which the user holds a permission.
  services: string[];
}

export interface FakeOrg {
  id: string;
  name: string;
  slug: string;
  member: boolean;
  entitled: boolean;
  projects: FakeProject[];
}

// world gives the organizations of the user in the order they joined: A and B
// use inventaris, C uses it without any project the user may use, and Z does
// not use it at all.
export function world(): FakeOrg[] {
  return [
    {
      id: ORG_A,
      name: "Nusapala",
      slug: "nusapala",
      member: true,
      entitled: true,
      projects: [
        { id: PRJ_P, name: "Parkir", slug: "parkir", services: [SERVICE] },
        { id: PRJ_Q, name: "Gudang", slug: "gudang", services: [SERVICE] },
      ],
    },
    {
      id: ORG_B,
      name: "Bintang Laut",
      slug: "bintang-laut",
      member: true,
      entitled: true,
      projects: [{ id: PRJ_R, name: "Retail", slug: "retail", services: [SERVICE] }],
    },
    { id: ORG_C, name: "Cakrawala", slug: "cakrawala", member: true, entitled: true, projects: [] },
    {
      id: "org_0000000d-0000-7000-8000-000000000000",
      name: "Dirgantara",
      slug: "dirgantara",
      member: true,
      entitled: false,
      projects: [],
    },
  ];
}

type Handler = (url: URL, init: RequestInit) => Response | Promise<Response> | undefined;

interface PendingCode {
  challenge: string;
  nonce: string;
  redirectUri: string;
  scope: string;
  org: string;
}

// FakeNice stands in for NICE with the rules a service client of inventaris
// meets: the first entitled organization at sign-in, nice:org on refresh with
// a check before rotation, nice:prj required for the service, a delegated
// aud=nice token, GET /api/v1/me/contexts, and grant scopes replaced by every
// refresh.
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
  orgs: FakeOrg[] = world();
  // sessionOrg is the organization the presented refresh token is bound to.
  sessionOrg = ORG_A;
  grantScopes: string[] = [...LOGIN_SCOPES];
  contextsPageSize?: number;

  readonly tokenRequests: URLSearchParams[] = [];
  readonly revokeRequests: URLSearchParams[] = [];
  readonly serviceRequests: { url: string; authorization: string | null }[] = [];
  readonly userinfoRequests: { authorization: string | null }[] = [];
  readonly contextsRequests: { url: string; authorization: string | null }[] = [];
  readonly urls: string[] = [];
  inFlight = 0;
  maxInFlight = 0;

  private rotations = 0;
  private accessCount = 0;
  private readonly codes = new Map<string, PendingCode>();
  private readonly accessScopes = new Map<string, string[]>();

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

  org(id: string): FakeOrg | undefined {
    return this.orgs.find((org) => org.id === id);
  }

  // authorize stands in for the NICE login page. Without nice:org it picks the
  // first organization of the user entitled to the bound service.
  authorize(authorizationUrl: string): string {
    const params = new URL(authorizationUrl).searchParams;
    const callback = new URL(params.get("redirect_uri") ?? REDIRECT_URI);
    callback.searchParams.set("state", params.get("state") ?? "");
    const org = this.orgs.find((candidate) => usable(candidate));
    if (!org) {
      callback.searchParams.set("error", "access_denied");
      callback.searchParams.set("error_description", "the service is not available in any of the user's organizations");
      return callback.href;
    }
    const code = `code-${this.codes.size + 1}`;
    this.codes.set(code, {
      challenge: params.get("code_challenge") ?? "",
      nonce: params.get("nonce") ?? "",
      redirectUri: params.get("redirect_uri") ?? "",
      scope: params.get("scope") ?? "",
      org: org.id,
    });
    callback.searchParams.set("code", code);
    return callback.href;
  }

  accessToken(aud: string, org: string, extra: Record<string, unknown> = {}): string {
    const iat = Math.floor(this.now() / 1000);
    this.accessCount += 1;
    this.accessScopes.set(`at-${this.accessCount}`, [...this.grantScopes]);
    return fakeJwt({
      iss: this.issuer,
      sub: SUBJECT,
      aud,
      azp: CLIENT_ID,
      iat,
      exp: iat + this.accessTtlSeconds,
      jti: `at-${this.accessCount}`,
      nice_org_id: org,
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
        return this.userinfo(new Headers(init.headers).get("authorization"));
      case "/oauth/revoke":
        this.revokeRequests.push(form(init));
        if (this.revokeAnswer === "network") throw new TypeError("Failed to fetch");
        return this.revokeAnswer === "ok" ? new Response(null, { status: 200 }) : json(503, { error: "server_error" });
      case "/api/v1/me/contexts":
        return this.contexts(url, new Headers(init.headers).get("authorization"));
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
    this.grantScopes = scopesOf(pending.scope);
    this.sessionOrg = pending.org;
    return json(200, {
      access_token: this.accessToken(CLIENT_ID, pending.org),
      token_type: "Bearer",
      expires_in: this.accessTtlSeconds,
      id_token: this.idToken({ nonce: this.idTokenNonce ?? pending.nonce }),
      ...(this.issueRefreshTokenOnLogin ? { refresh_token: this.rotate() } : {}),
    });
  }

  // Every refusal returns before rotate(), so the presented refresh token stays usable.
  private refresh(body: URLSearchParams): Response {
    if (body.get("client_id") !== CLIENT_ID) return json(400, { error: "invalid_client" });
    if (body.get("refresh_token") !== this.refreshToken) return json(400, { error: "invalid_grant" });
    const asked = body.has("scope") ? scopesOf(body.get("scope") ?? "") : this.grantScopes;
    const refuse = (description: string) => json(400, { error: "invalid_scope", error_description: description });
    const outside = asked.filter((s) => !this.grantScopes.includes(s) && !/^nice:(aud|prj|org):/.test(s));
    if (outside.length > 0) return refuse(`scope not granted: ${outside.join(" ")}`);
    const named = (prefix: string) => asked.filter((s) => s.startsWith(prefix)).map((s) => s.slice(prefix.length));
    const [orgs, auds, prjs] = [named("nice:org:"), named("nice:aud:"), named("nice:prj:")];
    if (orgs.length > 1 || auds.length > 1 || prjs.length > 1) return refuse("at most one nice:org, nice:aud, nice:prj");

    const orgId = orgs[0] ?? this.sessionOrg;
    if (orgs[0] !== undefined) {
      const target = this.org(orgId);
      if (!target || !usable(target)) {
        return json(400, { error: "access_denied", error_description: "the organization may not be used with this client" });
      }
    }
    const [aud, prj] = [auds[0], prjs[0]];
    let audience = CLIENT_ID;
    let extra: Record<string, unknown> = {};
    if (aud === "nice") {
      if (prj) return refuse("nice:prj is not allowed with nice:aud:nice");
      audience = "nice";
      extra = { perms: [CONTEXTS_PERMISSION], nice_service: SERVICE };
    } else if (aud === SERVICE) {
      if (!prj) return refuse("nice:prj is required for a service client");
      const project = this.org(orgId)?.projects.find((candidate) => candidate.id === prj);
      if (!project?.services.includes(SERVICE)) return refuse("service is not active in the project");
      audience = SERVICE;
      extra = { prj, perms: ["inventaris.items.read"] };
    } else if (aud !== undefined) {
      return refuse("audience is not allowed for this client");
    } else if (prj) {
      return refuse("nice:prj needs nice:aud");
    }

    this.grantScopes = asked;
    this.sessionOrg = orgId;
    const scope = asked.join(" ");
    return json(200, {
      access_token: this.accessToken(audience, orgId, { scope, ...extra }),
      token_type: "Bearer",
      expires_in: this.accessTtlSeconds,
      refresh_token: this.rotate(),
      scope,
    });
  }

  private userinfo(authorization: string | null): Response {
    this.userinfoRequests.push({ authorization });
    const claims = bearerClaims(authorization);
    const scopes = typeof claims?.jti === "string" ? this.accessScopes.get(claims.jti) : undefined;
    if (!scopes || claims?.aud !== CLIENT_ID) return json(401, { error: "invalid_token" });
    return json(200, {
      sub: SUBJECT,
      ...(scopes.includes("profile") ? { name: "Siti Rahma", preferred_username: EMAIL } : {}),
      ...(scopes.includes("email") ? { email: EMAIL, email_verified: false } : {}),
    });
  }

  private contexts(url: URL, authorization: string | null): Response {
    this.contextsRequests.push({ url: url.href, authorization });
    const claims = bearerClaims(authorization);
    if (!claims || claims.aud !== "nice") return apiError(401, "unauthenticated", "missing or invalid token");
    const perms = Array.isArray(claims.perms) ? claims.perms : [];
    if (!perms.includes(CONTEXTS_PERMISSION)) return apiError(403, "delegation_not_allowed", "not delegated");
    if (url.searchParams.has("service")) return apiError(400, "invalid_argument", "service is refused for a delegated token");
    const pageSize = Number(url.searchParams.get("page_size") ?? "20");
    if (!Number.isInteger(pageSize) || pageSize < 0 || pageSize > 50) return apiError(400, "invalid_argument", "page_size");
    const listed = this.orgs.filter((org) => usable(org));
    const token = url.searchParams.get("page_token") ?? "";
    const start = token ? listed.findIndex((org) => org.id === token) + 1 : 0;
    if (token && start === 0) return apiError(400, "invalid_page_token", "page_token");
    const size = Math.min(this.contextsPageSize ?? pageSize, pageSize || 20);
    const page = listed.slice(start, start + size);
    const more = start + size < listed.length;
    return json(200, {
      current_organization_id: claims.nice_org_id,
      services: [SERVICE],
      organizations: page.map((org) => ({
        id: org.id,
        name: org.name,
        slug: org.slug,
        appearance: {},
        projects: org.projects.map(({ id, name, slug, services }) => ({ id, name, slug, services })),
        projects_truncated: false,
      })),
      page: more ? { next_page_token: page.at(-1)?.id } : {},
    });
  }
}

const usable = (org: FakeOrg) => org.member && org.entitled;

// fakeInventaris answers GET /items with the items of the token's organization
// and project, so a test sees which context the call ran in.
export function fakeInventaris(): Handler {
  return (url, init) => {
    if (url.origin !== INVENTARIS_URL) return undefined;
    const claims = bearerClaims(new Headers(init.headers).get("authorization"));
    if (claims?.aud !== SERVICE) return json(401, { error: "invalid_token", error_description: "wrong_audience" });
    if (url.pathname !== "/items") return json(404, { error: "not_found" });
    return json(200, {
      items: [{ id: `itm-${String(claims.nice_org_id)}-${String(claims.prj)}`, name: `Barang ${String(claims.prj)}`, quantity: 3 }],
    });
  };
}

function bearerClaims(authorization: string | null): Record<string, unknown> | undefined {
  const token = authorization?.startsWith("Bearer ") ? authorization.slice(7) : "";
  try {
    return JSON.parse(atob(token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")));
  } catch {
    return undefined;
  }
}

function scopesOf(scope: string): string[] {
  return scope.split(" ").filter(Boolean);
}

export function json(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json", ...headers },
  });
}

function apiError(status: number, errorCode: string, message: string): Response {
  return json(status, { code: status, error_code: errorCode, message, request_id: "req-test" });
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
