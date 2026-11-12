import type { AppConfig, ServiceConfig } from "./config";
import { decodeClaims } from "./jwt";
import { OidcClient } from "./oidc";
import { clearSession, type KeyValueStorage, keys } from "./storage";
import { type TokenError, TokenManager, type TokenView } from "./token-manager";

export interface Identity {
  userinfo: Record<string, unknown>;
  orgId?: string;
}

export type Phase = "starting" | "unreachable" | "signed-out" | "redirecting" | "login-failed" | "signed-in" | "ended";

export interface SessionState {
  phase: Phase;
  message?: string;
  identity?: Identity;
  identityError?: string;
  loginAccessToken?: string;
  idToken?: string;
  serviceTokens: TokenView[];
}

export interface SessionDeps {
  storage: KeyValueStorage;
  fetch: typeof fetch;
  navigate: (url: string) => void;
  replaceUrl: (path: string) => void;
  now?: () => number;
  random?: () => number;
}

export class Session {
  private state: SessionState = { phase: "starting", serviceTokens: [] };
  private readonly listeners = new Set<() => void>();
  private started?: Promise<void>;
  private oidc?: OidcClient;
  private tokens?: TokenManager;

  constructor(
    readonly config: AppConfig,
    private readonly deps: SessionDeps,
  ) {}

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  readonly getState = (): SessionState => this.state;

  // completeLogin and resume run once per page load, whichever comes first.
  completeLogin(href: string): Promise<void> {
    this.started ??= this.runCallback(href);
    return this.started;
  }

  resume(): Promise<void> {
    this.started ??= this.runResume();
    return this.started;
  }

  retry(): Promise<void> {
    this.started = this.runResume();
    return this.started;
  }

  async signIn(): Promise<void> {
    const oidc = await this.connect();
    if (!oidc) return;
    this.set({ phase: "redirecting" });
    try {
      this.deps.navigate(await oidc.authorizationUrl(this.deps.storage));
    } catch (error) {
      this.set({ phase: "login-failed", message: `Login tidak dapat dimulai: ${describe(error)}` });
    }
  }

  call(service: ServiceConfig): Promise<Response> {
    if (!this.tokens) return Promise.reject(new Error("Belum login."));
    return this.tokens.call(service, service.endpoint + service.path, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
  }

  async logout(): Promise<void> {
    const oidc = this.oidc ?? (await this.connect());
    if (!oidc) return;
    this.set({ phase: "redirecting" });
    const { storage } = this.deps;
    await this.tokens?.close();
    const refreshToken = storage.getItem(keys.refreshToken);
    const idToken = storage.getItem(keys.idToken) ?? undefined;
    if (refreshToken) {
      try {
        await oidc.revoke(refreshToken);
      } catch {
        // A failed revocation must not keep the session in this tab; the token family still ends at end-session.
      }
    }
    clearSession(storage);
    this.deps.navigate(oidc.endSessionUrl(idToken));
  }

  private async runCallback(href: string): Promise<void> {
    // The code and state leave the address bar and history before anything else.
    this.deps.replaceUrl("/");
    const oidc = await this.connect();
    if (!oidc) return;
    const { storage } = this.deps;
    let outcome: Awaited<ReturnType<OidcClient["completeLogin"]>>;
    try {
      outcome = await oidc.completeLogin(new URL(href), storage);
    } catch (error) {
      this.set({ phase: "login-failed", message: describe(error) });
      return;
    }
    if (outcome.kind === "restart") {
      await this.signIn();
      return;
    }
    try {
      storage.setItem(keys.refreshToken, outcome.refreshToken);
      storage.setItem(keys.idToken, outcome.idToken);
    } catch (error) {
      clearSession(storage);
      this.set({ phase: "login-failed", message: `Sesi tidak dapat disimpan di sessionStorage: ${describe(error)}` });
      return;
    }
    this.startTokens(oidc);
    this.set({ phase: "signed-in", loginAccessToken: outcome.accessToken, idToken: outcome.idToken });
    try {
      const userinfo = await oidc.userinfo(outcome.accessToken, outcome.subject);
      const orgId = decodeClaims(outcome.accessToken)?.nice_org_id;
      const identity: Identity = { userinfo: { ...userinfo }, ...(typeof orgId === "string" ? { orgId } : {}) };
      storage.setItem(keys.identity, JSON.stringify(identity));
      this.set({ identity });
    } catch (error) {
      this.set({ identityError: `Userinfo tidak dapat dibaca: ${describe(error)}` });
    }
  }

  private async runResume(): Promise<void> {
    this.set({ phase: "starting", message: undefined });
    const oidc = await this.connect();
    if (!oidc) return;
    const { storage } = this.deps;
    if (!storage.getItem(keys.refreshToken)) {
      this.set({ phase: "signed-out" });
      return;
    }
    this.startTokens(oidc);
    this.set({
      phase: "signed-in",
      idToken: storage.getItem(keys.idToken) ?? undefined,
      identity: readIdentity(storage.getItem(keys.identity)),
    });
  }

  private async connect(): Promise<OidcClient | undefined> {
    if (this.oidc) return this.oidc;
    try {
      this.oidc = await OidcClient.discover({
        issuer: this.config.issuer,
        clientId: this.config.clientId,
        redirectUri: this.config.redirectUri,
        postLogoutRedirectUri: this.config.postLogoutRedirectUri,
        fetch: this.deps.fetch,
      });
      return this.oidc;
    } catch (error) {
      this.set({
        phase: "unreachable",
        message: `Discovery NICE di ${this.config.issuer} gagal: ${describe(error)}`,
      });
      return undefined;
    }
  }

  private startTokens(oidc: OidcClient): void {
    const tokens = new TokenManager({
      storage: this.deps.storage,
      grant: oidc.refreshGrant,
      fetch: this.deps.fetch,
      now: this.deps.now,
      random: this.deps.random,
      onSessionEnded: (error) => this.ended(error),
    });
    tokens.subscribe(() => this.set({ serviceTokens: tokens.snapshot() }));
    this.tokens = tokens;
  }

  private ended(error: TokenError): void {
    clearSession(this.deps.storage);
    this.tokens = undefined;
    this.state = { phase: "ended", message: error.message, serviceTokens: [] };
    this.emit();
  }

  private set(patch: Partial<SessionState>): void {
    this.state = { ...this.state, ...patch };
    this.emit();
  }

  private emit(): void {
    for (const listener of this.listeners) listener();
  }
}

function readIdentity(raw: string | null): Identity | undefined {
  if (!raw) return undefined;
  try {
    const value = JSON.parse(raw) as Identity | null;
    return value && typeof value.userinfo === "object" ? value : undefined;
  } catch {
    return undefined;
  }
}

function describe(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
