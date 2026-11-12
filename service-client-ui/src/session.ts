import type { AppConfig } from "./config";
import { type ActiveContext, type Contexts, loadContexts } from "./contexts";
import { decodeClaims } from "./jwt";
import { LoginError, type LoginErrorReason, OidcClient } from "./oidc";
import { clearSession, type KeyValueStorage, keys } from "./storage";
import { TokenError, TokenManager, type TokenView } from "./token-manager";

export interface Identity {
  userinfo: Record<string, unknown>;
  orgId?: string;
}

export type Phase = "starting" | "unreachable" | "signed-out" | "redirecting" | "login-failed" | "signed-in" | "ended";

export interface Item {
  id: string;
  name: string;
  quantity: number;
}

export interface SessionState {
  phase: Phase;
  message?: string;
  reason?: LoginErrorReason;
  identity?: Identity;
  identityError?: string;
  idToken?: string;
  tokens: TokenView[];
  // active is the context the UI shows; it lives in memory only.
  active?: ActiveContext;
  contexts?: Contexts;
  contextsError?: string;
  contextsLoading?: boolean;
  // switching is the context being asked from NICE; active changes only when NICE agrees.
  switching?: ActiveContext;
  switchError?: string;
  items?: Item[];
  itemsError?: string;
  itemsLoading?: boolean;
}

export interface SessionDeps {
  storage: KeyValueStorage;
  fetch: typeof fetch;
  navigate: (url: string) => void;
  replaceUrl: (path: string) => void;
  now?: () => number;
  random?: () => number;
}

// The client token: no service, aud = client_id, only for /oauth/userinfo.
const CLIENT = {};
const SERVICE = "inventaris";

export class Session {
  private state: SessionState = { phase: "starting", tokens: [] };
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

  async signIn(options: { selectAccount?: boolean } = {}): Promise<void> {
    const oidc = await this.connect();
    if (!oidc) return;
    this.set({ phase: "redirecting", reason: undefined });
    try {
      this.deps.navigate(await oidc.authorizationUrl(this.deps.storage, options));
    } catch (error) {
      this.set({ phase: "login-failed", message: `Login tidak dapat dimulai: ${describe(error)}` });
    }
  }

  async reloadContexts(): Promise<void> {
    const tokens = this.tokens;
    if (!tokens) return;
    this.set({ contextsLoading: true });
    try {
      const contexts = await loadContexts(tokens, this.config.issuer, this.state.active?.org);
      if (this.tokens !== tokens) return;
      this.set({ contexts, contextsError: undefined, contextsLoading: false });
    } catch (error) {
      if (this.tokens !== tokens || isEnded(error)) return;
      this.set({ contextsError: `Konteks tidak dapat dibaca: ${describe(error)}`, contextsLoading: false });
    }
  }

  // choose moves the UI to a context with one refresh (nice:org when the
  // organization changes, nice:aud, nice:prj), or none when a token for it is
  // still held. A refusal keeps the previous context and the session.
  async choose(org: string, project: string): Promise<void> {
    const tokens = this.tokens;
    if (!tokens) return;
    this.set({ switching: { org, project }, switchError: undefined });
    try {
      await tokens.token({ org, service: SERVICE, project });
    } catch (error) {
      if (this.tokens !== tokens || isEnded(error)) return;
      this.set({ switching: undefined, switchError: `Konteks itu tidak dapat dipakai, Anda tetap di konteks sebelumnya. ${describe(error)}` });
      // What /me/contexts answered earlier is stale: the access changed since.
      await this.reloadContexts();
      return;
    }
    if (this.tokens !== tokens) return;
    this.set({ switching: undefined, active: { org, project }, items: undefined, itemsError: undefined });
    await this.loadItems();
  }

  async loadItems(): Promise<void> {
    const tokens = this.tokens;
    const active = this.state.active;
    if (!tokens || !active?.project) return;
    this.set({ itemsLoading: true });
    try {
      const response = await tokens.call({ org: active.org, service: SERVICE, project: active.project }, `${this.config.inventarisUrl}/items`, {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      const body = (await response.json().catch(() => undefined)) as { items?: unknown; error?: unknown } | undefined;
      if (this.tokens !== tokens || this.state.active !== active) return;
      if (!response.ok || !Array.isArray(body?.items)) {
        const code = typeof body?.error === "string" ? ` ${body.error}` : "";
        this.set({ items: undefined, itemsError: `inventaris menjawab HTTP ${response.status}${code}.`, itemsLoading: false });
        return;
      }
      this.set({ items: body.items.map(item), itemsError: undefined, itemsLoading: false });
    } catch (error) {
      if (this.tokens !== tokens || isEnded(error)) return;
      this.set({
        itemsError: `inventaris tidak dapat dipanggil: ${describe(error)}. Pastikan service berjalan dan CORS-nya mengizinkan origin UI ini.`,
        itemsLoading: false,
      });
    }
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
      this.set({ phase: "login-failed", message: describe(error), reason: error instanceof LoginError ? error.reason : "refused" });
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
    const tokens = this.startTokens(oidc);
    // The access token of the code exchange has aud = client_id, the base
    // scope, and the organization NICE chose, so it is the first client token.
    tokens.hold(CLIENT, outcome.accessToken, outcome.expiresIn);
    this.set({ phase: "signed-in", idToken: outcome.idToken });
    await this.loadIdentity(oidc, tokens, outcome.subject);
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
    const tokens = this.startTokens(oidc);
    const idToken = storage.getItem(keys.idToken) ?? undefined;
    this.set({ phase: "signed-in", idToken });
    const subject = idToken ? decodeClaims(idToken)?.sub : undefined;
    await this.loadIdentity(oidc, tokens, typeof subject === "string" ? subject : undefined);
  }

  // loadIdentity asks userinfo with the client token, takes the organization
  // of that token as the current one, then reads the contexts.
  private async loadIdentity(oidc: OidcClient, tokens: TokenManager, subject: string | undefined): Promise<void> {
    try {
      const access = await tokens.token(CLIENT);
      const claims = decodeClaims(access);
      const orgId = typeof claims?.nice_org_id === "string" ? claims.nice_org_id : undefined;
      if (orgId) this.set({ active: { org: orgId } });
      const userinfo = await oidc.userinfo(access, subject ?? (typeof claims?.sub === "string" ? claims.sub : undefined));
      if (this.tokens !== tokens) return;
      this.set({ identity: { userinfo: { ...userinfo }, ...(orgId ? { orgId } : {}) }, identityError: undefined });
    } catch (error) {
      if (this.tokens !== tokens || isEnded(error)) return;
      this.set({ identityError: `Userinfo tidak dapat dibaca: ${describe(error)}` });
    }
    if (this.state.active) await this.reloadContexts();
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
      this.set({ phase: "unreachable", message: `Discovery NICE di ${this.config.issuer} gagal: ${describe(error)}` });
      return undefined;
    }
  }

  private startTokens(oidc: OidcClient): TokenManager {
    const tokens = new TokenManager({
      storage: this.deps.storage,
      grant: oidc.refreshGrant,
      fetch: this.deps.fetch,
      now: this.deps.now,
      random: this.deps.random,
      onSessionEnded: (error) => this.ended(error),
    });
    tokens.subscribe(() => this.set({ tokens: tokens.snapshot() }));
    this.tokens = tokens;
    return tokens;
  }

  private ended(error: TokenError): void {
    clearSession(this.deps.storage);
    this.tokens = undefined;
    this.state = { phase: "ended", message: error.message, tokens: [] };
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

function item(raw: unknown): Item {
  const value = (raw ?? {}) as Record<string, unknown>;
  return {
    id: typeof value.id === "string" ? value.id : "",
    name: typeof value.name === "string" ? value.name : "",
    quantity: typeof value.quantity === "number" ? value.quantity : 0,
  };
}

// A session that ended already shows "Sesi berakhir"; its errors need no other message.
function isEnded(error: unknown): boolean {
  return error instanceof TokenError && error.kind === "session_ended";
}

function describe(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
