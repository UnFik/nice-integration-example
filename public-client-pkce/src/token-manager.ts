import { type Claims, decodeClaims, tokenTimes } from "./jwt";
import { type KeyValueStorage, keys } from "./storage";

export interface ServiceKey {
  slug: string;
  project?: string;
}

export const keyOf = (service: ServiceKey): string => `${service.slug}|${service.project ?? ""}`;

export const scopeOf = (service: ServiceKey): string =>
  `nice:aud:${service.slug}` + (service.project ? ` nice:prj:${service.project}` : "");

export type TokenErrorKind = "temporary" | "refused" | "session_ended";

export class TokenError extends Error {
  constructor(
    readonly kind: TokenErrorKind,
    message: string,
    readonly code?: string,
    readonly description?: string,
    readonly status?: number,
  ) {
    super(message);
    this.name = "TokenError";
  }
}

export interface GrantResponse {
  readonly access_token: string;
  readonly expires_in?: number;
}

// keep must be called with the rotated refresh token as soon as it is known,
// before the rest of the answer is checked: the old one is already spent.
export type RefreshGrant = (
  refreshToken: string,
  scope: string,
  keep: (rotated: string) => void,
) => Promise<GrantResponse>;

export interface TokenView {
  key: string;
  service: ServiceKey;
  access: string;
  claims: Claims | undefined;
  issuedAt: number;
  expiresAt: number;
  refreshAt: number;
}

export interface TokenManagerOptions {
  storage: KeyValueStorage;
  grant: RefreshGrant;
  fetch: typeof fetch;
  now?: () => number;
  random?: () => number;
  onSessionEnded?: (error: TokenError) => void;
  refreshAfterMs?: number;
  jitterMs?: number;
  safetyMs?: number;
  minBackoffMs?: number;
  maxBackoffMs?: number;
}

interface Held {
  access: string;
  issuedAt: number;
  expiresAt: number;
  refreshAt: number;
}

interface Entry {
  service: ServiceKey;
  token?: Held;
  inflight?: Promise<string>;
  retryAt: number;
  backoff: number;
  lastError?: TokenError;
}

// TokenManager keeps one access token per (service, project) for the signed-in
// user and gets them with the refresh token, like pkg/niceclient in NICE.
export class TokenManager {
  private readonly storage: KeyValueStorage;
  private readonly grant: RefreshGrant;
  private readonly fetchImpl: typeof fetch;
  private readonly now: () => number;
  private readonly random: () => number;
  private readonly onSessionEnded?: (error: TokenError) => void;
  private readonly refreshAfter: number;
  private readonly jitter: number;
  private readonly safety: number;
  private readonly minBackoff: number;
  private readonly maxBackoff: number;

  private readonly entries = new Map<string, Entry>();
  private readonly listeners = new Set<() => void>();
  // NICE rotates the refresh token on every use, so refreshes run one at a time.
  private queue: Promise<unknown> = Promise.resolve();
  private ended?: TokenError;
  private closed = false;
  private view: TokenView[] = [];

  constructor(options: TokenManagerOptions) {
    this.storage = options.storage;
    this.grant = options.grant;
    this.fetchImpl = options.fetch;
    this.now = options.now ?? Date.now;
    this.random = options.random ?? Math.random;
    this.onSessionEnded = options.onSessionEnded;
    this.refreshAfter = options.refreshAfterMs ?? 10 * 60_000;
    this.jitter = options.jitterMs ?? 60_000;
    this.safety = options.safetyMs ?? 60_000;
    this.minBackoff = options.minBackoffMs ?? 5_000;
    this.maxBackoff = options.maxBackoffMs ?? 60_000;
  }

  async token(service: ServiceKey): Promise<string> {
    this.assertOpen();
    const entry = this.entryFor(service);
    const now = this.now();
    const held = entry.token;
    const valid = held !== undefined && now < held.expiresAt - this.safety;
    if (valid && (now < held.refreshAt || now < entry.retryAt)) return held.access;
    if (!entry.inflight) {
      if (!valid && entry.lastError && now < entry.retryAt) throw entry.lastError;
      entry.inflight = this.renew(entry).finally(() => {
        entry.inflight = undefined;
      });
    }
    return entry.inflight;
  }

  // call sends a request with the token of service. A 401 token_stale or
  // expired gets one new token and one more attempt; the second answer is
  // returned whatever it is.
  async call(service: ServiceKey, url: string, init: RequestInit = {}): Promise<Response> {
    const access = await this.token(service);
    const response = await this.send(url, init, access);
    if (response.status !== 401 || !canResend(init.body) || !(await isStale(response))) return response;
    this.invalidate(service, access);
    return this.send(url, init, await this.token(service));
  }

  invalidate(service: ServiceKey, access: string): void {
    const entry = this.entries.get(keyOf(service));
    if (entry?.token?.access !== access) return;
    entry.token = undefined;
    entry.retryAt = 0;
    entry.backoff = 0;
    entry.lastError = undefined;
    this.changed();
  }

  snapshot(): TokenView[] {
    return this.view;
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  // close stops new refreshes and waits for the running one, so the refresh
  // token read afterwards is the latest one.
  async close(): Promise<void> {
    this.closed = true;
    this.entries.clear();
    this.changed();
    await this.queue;
  }

  private assertOpen(): void {
    if (this.ended) throw this.ended;
    if (this.closed) throw new TokenError("session_ended", "Sesi sudah ditutup.");
  }

  private entryFor(service: ServiceKey): Entry {
    const key = keyOf(service);
    let entry = this.entries.get(key);
    if (!entry) {
      entry = { service: { slug: service.slug, project: service.project }, retryAt: 0, backoff: 0 };
      this.entries.set(key, entry);
    }
    return entry;
  }

  private async renew(entry: Entry): Promise<string> {
    try {
      const fresh = await this.enqueue(() => this.refresh(entry.service));
      entry.token = fresh;
      entry.retryAt = 0;
      entry.backoff = 0;
      entry.lastError = undefined;
      this.changed();
      return fresh.access;
    } catch (cause) {
      const error =
        cause instanceof TokenError
          ? cause
          : new TokenError("refused", `Token tidak dapat diambil: ${cause instanceof Error ? cause.message : String(cause)}`);
      if (error.kind === "session_ended") {
        this.endSession(error);
        throw error;
      }
      if (error.kind === "temporary") {
        entry.backoff = entry.backoff ? Math.min(entry.backoff * 2, this.maxBackoff) : this.minBackoff;
        entry.retryAt = this.now() + entry.backoff;
        entry.lastError = error;
        const held = entry.token;
        if (held && this.now() < held.expiresAt - this.safety) return held.access;
        throw error;
      }
      entry.token = undefined;
      entry.retryAt = 0;
      entry.backoff = 0;
      entry.lastError = error;
      this.changed();
      throw error;
    }
  }

  private enqueue<T>(task: () => Promise<T>): Promise<T> {
    const run = this.queue.then(task);
    this.queue = run.catch(() => undefined);
    return run;
  }

  // refresh runs inside the queue and reads the refresh token only then, so
  // each request carries the one the previous answer left behind.
  private async refresh(service: ServiceKey): Promise<Held> {
    this.assertOpen();
    const refreshToken = this.storage.getItem(keys.refreshToken);
    if (!refreshToken) throw new TokenError("session_ended", "Tidak ada refresh token di sessionStorage.");
    const response = await this.grant(refreshToken, scopeOf(service), (rotated) => this.keep(rotated));
    const now = this.now();
    const times = tokenTimes(response.access_token) ?? {
      issuedAt: now,
      expiresAt: now + (response.expires_in ?? 0) * 1000,
    };
    return {
      access: response.access_token,
      ...times,
      refreshAt: times.issuedAt + this.refreshAfter + this.random() * this.jitter,
    };
  }

  private keep(rotated: string): void {
    try {
      this.storage.setItem(keys.refreshToken, rotated);
    } catch {
      throw new TokenError("session_ended", "Refresh token baru tidak dapat disimpan, sehingga sesi hilang.");
    }
  }

  private endSession(error: TokenError): void {
    if (this.ended || this.closed) return;
    this.ended = error;
    this.entries.clear();
    this.storage.removeItem(keys.refreshToken);
    this.changed();
    this.onSessionEnded?.(error);
  }

  private send(url: string, init: RequestInit, access: string): Promise<Response> {
    const headers = new Headers(init.headers);
    headers.set("Authorization", `Bearer ${access}`);
    return this.fetchImpl(url, { ...init, headers });
  }

  private changed(): void {
    this.view = [...this.entries.entries()].flatMap(([key, entry]) =>
      entry.token
        ? [
            {
              key,
              service: entry.service,
              access: entry.token.access,
              claims: decodeClaims(entry.token.access),
              issuedAt: entry.token.issuedAt,
              expiresAt: entry.token.expiresAt,
              refreshAt: entry.token.refreshAt,
            },
          ]
        : [],
    );
    for (const listener of this.listeners) listener();
  }
}

function canResend(body: RequestInit["body"]): boolean {
  return !(typeof ReadableStream !== "undefined" && body instanceof ReadableStream);
}

// Browsers only expose WWW-Authenticate when the service lists it in
// Access-Control-Expose-Headers, so the JSON body is read as well.
async function isStale(response: Response): Promise<boolean> {
  const challenge = response.headers.get("WWW-Authenticate") ?? "";
  if (challenge.includes("token_stale") || challenge.includes('error_description="expired"')) return true;
  try {
    const body: unknown = await response.clone().json();
    const description = (body as { error_description?: unknown } | null)?.error_description;
    return description === "token_stale" || description === "expired";
  } catch {
    return false;
  }
}
