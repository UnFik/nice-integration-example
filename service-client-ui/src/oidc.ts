import * as oauth from "oauth4webapi";
import { BASE_SCOPE } from "./scope";
import { type KeyValueStorage, keys } from "./storage";
import { type GrantResponse, type RefreshGrant, TokenError } from "./token-manager";

const LOOPBACK_HOSTS = new Set(["localhost", "127.0.0.1", "[::1]"]);
const PERMANENT_GRANT_ERRORS = new Set(["invalid_grant", "invalid_client", "unauthorized_client"]);

export class InsecureIssuerError extends Error {
  constructor(issuer: URL) {
    super(`Issuer ${issuer.origin} harus memakai https; http hanya diizinkan untuk localhost, 127.0.0.1, atau [::1].`);
    this.name = "InsecureIssuerError";
  }
}

// service_unavailable is NICE's access_denied for a service client: none of
// the user's organizations is entitled to the bound service.
export type LoginErrorReason = "service_unavailable" | "refused";

export class LoginError extends Error {
  constructor(
    message: string,
    readonly reason: LoginErrorReason = "refused",
  ) {
    super(message);
    this.name = "LoginError";
  }
}

// allowInsecureRequests is only for a dev NICE on this machine; any other
// http issuer is refused before a request is sent.
export function issuerTransport(issuer: URL): { allowInsecure: boolean } {
  if (issuer.protocol === "https:") return { allowInsecure: false };
  if (issuer.protocol === "http:" && LOOPBACK_HOSTS.has(issuer.hostname)) return { allowInsecure: true };
  throw new InsecureIssuerError(issuer);
}

export interface OidcOptions {
  issuer: string;
  clientId: string;
  redirectUri: string;
  postLogoutRedirectUri: string;
  fetch?: typeof fetch;
}

interface LoginTransaction {
  state: string;
  nonce: string;
  verifier: string;
}

export type CallbackOutcome =
  | { kind: "restart" }
  | {
      kind: "signed-in";
      accessToken: string;
      expiresIn?: number;
      idToken: string;
      refreshToken: string;
      subject: string;
    };

type HttpOptions = {
  [oauth.customFetch]: (url: string, options: oauth.CustomFetchOptions<string, unknown>) => Promise<Response>;
  [oauth.allowInsecureRequests]: boolean;
  signal: () => AbortSignal;
};

export class OidcClient {
  private readonly client: oauth.Client;

  private constructor(
    readonly as: oauth.AuthorizationServer,
    private readonly options: OidcOptions,
    private readonly http: HttpOptions,
  ) {
    this.client = { client_id: options.clientId };
  }

  static async discover(options: OidcOptions): Promise<OidcClient> {
    const issuer = new URL(options.issuer);
    const { allowInsecure } = issuerTransport(issuer);
    const fetchImpl = options.fetch ?? globalThis.fetch.bind(globalThis);
    const http: HttpOptions = {
      [oauth.customFetch]: (url, init) => fetchImpl(url, init as RequestInit),
      [oauth.allowInsecureRequests]: allowInsecure,
      signal: () => AbortSignal.timeout(30_000),
    };
    const response = await oauth.discoveryRequest(issuer, http);
    return new OidcClient(await oauth.processDiscoveryResponse(issuer, response), options, http);
  }

  async authorizationUrl(storage: KeyValueStorage, options: { selectAccount?: boolean } = {}): Promise<string> {
    if (!this.as.authorization_endpoint) throw new LoginError("Discovery NICE tidak memuat authorization_endpoint.");
    const transaction: LoginTransaction = {
      state: oauth.generateRandomState(),
      nonce: oauth.generateRandomNonce(),
      verifier: oauth.generateRandomCodeVerifier(),
    };
    const url = new URL(this.as.authorization_endpoint);
    const params = {
      client_id: this.options.clientId,
      redirect_uri: this.options.redirectUri,
      response_type: "code",
      scope: BASE_SCOPE,
      state: transaction.state,
      nonce: transaction.nonce,
      code_challenge: await oauth.calculatePKCECodeChallenge(transaction.verifier),
      code_challenge_method: "S256",
    };
    for (const [name, value] of Object.entries(params)) url.searchParams.set(name, value);
    if (options.selectAccount) url.searchParams.set("prompt", "select_account");
    storage.setItem(keys.login, JSON.stringify(transaction));
    return url.href;
  }

  async completeLogin(callback: URL, storage: KeyValueStorage): Promise<CallbackOutcome> {
    const transaction = parseTransaction(storage.getItem(keys.login));
    // The transaction is single use: a state seen once is never accepted again.
    storage.removeItem(keys.login);
    if (!transaction) {
      throw new LoginError("Permintaan login ini tidak dibuat oleh tab ini atau sudah dipakai. Kode tidak ditukar.");
    }
    let params: URLSearchParams;
    try {
      params = oauth.validateAuthResponse(this.as, this.client, callback.searchParams, transaction.state);
    } catch (error) {
      if (error instanceof oauth.AuthorizationResponseError) {
        if (error.error === "temporarily_unavailable" && error.error_description === "authentication_expired") {
          return { kind: "restart" };
        }
        if (error.error === "access_denied") {
          throw new LoginError(
            "Tidak ada organisasi Anda yang memakai service ini, atau organisasinya tidak aktif. Masuk dengan akun lain, atau minta admin organisasi mengaktifkan service ini.",
            "service_unavailable",
          );
        }
        throw new LoginError(
          `NICE menolak login: ${error.error}${error.error_description ? ` (${error.error_description})` : ""}.`,
        );
      }
      throw new LoginError(`Jawaban login ditolak, kode tidak ditukar: ${describe(error)}.`);
    }
    let result: oauth.TokenEndpointResponse;
    try {
      const response = await oauth.authorizationCodeGrantRequest(
        this.as,
        this.client,
        oauth.None(),
        params,
        this.options.redirectUri,
        transaction.verifier,
        this.http,
      );
      result = await oauth.processAuthorizationCodeResponse(this.as, this.client, response, {
        expectedNonce: transaction.nonce,
        requireIdToken: true,
      });
    } catch (error) {
      throw new LoginError(`Penukaran kode gagal: ${describe(error)}.`);
    }
    const claims = oauth.getValidatedIdTokenClaims(result);
    if (!claims || !result.id_token) throw new LoginError("NICE tidak mengirim ID token.");
    if (!result.refresh_token) {
      throw new LoginError(
        "NICE tidak mengirim refresh token. Pastikan OAuth client memiliki grant refresh_token dan scope memuat offline_access.",
      );
    }
    return {
      kind: "signed-in",
      accessToken: result.access_token,
      expiresIn: result.expires_in,
      idToken: result.id_token,
      refreshToken: result.refresh_token,
      subject: claims.sub,
    };
  }

  readonly refreshGrant: RefreshGrant = async (refreshToken, scope, keep) => {
    let response: Response;
    try {
      response = await oauth.refreshTokenGrantRequest(this.as, this.client, oauth.None(), refreshToken, {
        ...this.http,
        additionalParameters: { scope },
      });
    } catch (error) {
      throw new TokenError("temporary", `NICE tidak terjangkau: ${describe(error)}.`);
    }
    if (response.status === 200) {
      const rotated = await rotatedRefreshToken(response);
      if (rotated) keep(rotated);
    }
    try {
      const result: GrantResponse = await oauth.processRefreshTokenResponse(this.as, this.client, response);
      return result;
    } catch (error) {
      throw await grantFailure(error);
    }
  };

  async userinfo(accessToken: string, subject: string | undefined): Promise<oauth.UserInfoResponse> {
    const response = await oauth.userInfoRequest(this.as, this.client, accessToken, this.http);
    return oauth.processUserInfoResponse(this.as, this.client, subject ?? oauth.skipSubjectCheck, response);
  }

  async revoke(refreshToken: string): Promise<void> {
    const response = await oauth.revocationRequest(this.as, this.client, oauth.None(), refreshToken, {
      ...this.http,
      signal: () => AbortSignal.timeout(5_000),
      additionalParameters: { token_type_hint: "refresh_token" },
    });
    await oauth.processRevocationResponse(response);
  }

  endSessionUrl(idToken: string | undefined): string {
    if (!this.as.end_session_endpoint) return this.options.postLogoutRedirectUri;
    const url = new URL(this.as.end_session_endpoint);
    if (idToken) url.searchParams.set("id_token_hint", idToken);
    url.searchParams.set("client_id", this.options.clientId);
    url.searchParams.set("post_logout_redirect_uri", this.options.postLogoutRedirectUri);
    return url.href;
  }
}

function parseTransaction(raw: string | null): LoginTransaction | undefined {
  if (!raw) return undefined;
  try {
    const value = JSON.parse(raw) as Partial<LoginTransaction> | null;
    if (typeof value?.state === "string" && typeof value.nonce === "string" && typeof value.verifier === "string") {
      return { state: value.state, nonce: value.nonce, verifier: value.verifier };
    }
  } catch {
    return undefined;
  }
  return undefined;
}

async function rotatedRefreshToken(response: Response): Promise<string | undefined> {
  try {
    const body: unknown = await response.clone().json();
    const token = (body as { refresh_token?: unknown } | null)?.refresh_token;
    return typeof token === "string" && token !== "" ? token : undefined;
  } catch {
    return undefined;
  }
}

// grantFailure sorts a refused refresh the way pkg/niceclient does: network,
// 5xx and 429 are worth retrying; invalid_grant and invalid_client end the session.
async function grantFailure(error: unknown): Promise<TokenError> {
  let status: number | undefined;
  let code: string | undefined;
  let description: string | undefined;
  if (error instanceof oauth.ResponseBodyError) {
    ({ status, error: code, error_description: description } = error);
  } else if (error instanceof oauth.WWWAuthenticateChallengeError) {
    status = error.status;
    const body = (await error.response.json().catch(() => undefined)) as
      | { error?: unknown; error_description?: unknown }
      | undefined;
    const challenge = error.cause[0]?.parameters;
    code = typeof body?.error === "string" ? body.error : challenge?.error;
    description = typeof body?.error_description === "string" ? body.error_description : challenge?.error_description;
  } else if (error instanceof oauth.OperationProcessingError && error.cause instanceof Response) {
    status = error.cause.status;
  } else {
    return new TokenError("refused", `Jawaban token tidak sah: ${describe(error)}.`);
  }
  code ??= `http_${status}`;
  const text = `NICE menolak permintaan token: ${code}${description ? ` (${description})` : ""}.`;
  if (status >= 500 || status === 429) return new TokenError("temporary", text, code, description, status);
  if (PERMANENT_GRANT_ERRORS.has(code)) return new TokenError("session_ended", text, code, description, status);
  return new TokenError("refused", text, code, description, status);
}

function describe(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
