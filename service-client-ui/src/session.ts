import type { AppConfig } from "./config";
import { type ActiveContext, type Contexts, loadContexts } from "./contexts";
import { decodeClaims } from "./jwt";
import { describeItemsError, type Item, type ItemAction, type ItemInput, ItemsApiError, itemsApi } from "./items";
import { LoginError, type LoginErrorReason, OidcClient } from "./oidc";
import { readRecent, remember } from "./recent-contexts";
import { clearSession, type KeyValueStorage, keys } from "./storage";
import { TokenError, TokenManager, type TokenView } from "./token-manager";

export interface Identity {
  userinfo: Record<string, unknown>;
  orgId?: string;
}

export type Phase = "starting" | "unreachable" | "signed-out" | "redirecting" | "login-failed" | "signed-in" | "ended";

export type { Item } from "./items";

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
  // recent are manually chosen contexts that worked in this tab: ids only, never tokens.
  recent?: ActiveContext[];
  items?: Item[];
  itemsError?: string;
  itemsLoading?: boolean;
}

export type ItemResult = { ok: true } | { ok: false; message: string };

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
  ) {
    this.state = { ...this.state, recent: readRecent(deps.storage) };
  }

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
  async choose(org: string, project: string): Promise<boolean> {
    const tokens = this.tokens;
    if (!tokens) return false;
    this.set({ switching: { org, project }, switchError: undefined });
    try {
      await tokens.token({ org, service: SERVICE, project });
    } catch (error) {
      if (this.tokens !== tokens || isEnded(error)) return false;
      this.set({ switching: undefined, switchError: `Konteks itu tidak dapat dipakai, Anda tetap di konteks sebelumnya. ${switchReason(error)}` });
      // What /me/contexts answered earlier is stale: the access changed since.
      if (this.state.contexts) await this.reloadContexts();
      return false;
    }
    if (this.tokens !== tokens) return false;
    this.set({ switching: undefined, active: { org, project }, items: undefined, itemsError: undefined });
    await this.loadItems();
    return true;
  }

  // chooseManual is choose for ids typed by the user, kept in sessionStorage
  // only once NICE accepted them.
  async chooseManual(org: string, project: string): Promise<boolean> {
    const accepted = await this.choose(org, project);
    if (accepted) this.set({ recent: remember(this.deps.storage, { org, project }) });
    return accepted;
  }

  async loadItems(): Promise<void> {
    const tokens = this.tokens;
    const active = this.state.active;
    if (!tokens || !active?.project) return;
    this.set({ itemsLoading: true });
    try {
      const items = await this.api(tokens, active).list();
      if (this.tokens !== tokens || this.state.active !== active) return;
      this.set({ items, itemsError: undefined, itemsLoading: false });
    } catch (error) {
      if (this.tokens !== tokens || isEnded(error)) return;
      if (this.state.active !== active) return;
      const status = error instanceof ItemsApiError ? ` (HTTP ${error.status})` : "";
      this.set({ items: undefined, itemsError: `${describeItemsError(error, "read")}${status}`, itemsLoading: false });
    }
  }

  // saveItem creates an item, or changes the one with id, then reloads the list.
  // The answer is a sentence for the user; the list is only touched on success.
  saveItem(input: ItemInput, id?: string): Promise<ItemResult> {
    return this.mutate(id ? "edit" : "create", (api) => (id ? api.update(id, input) : api.create(input)));
  }

  removeItem(id: string): Promise<ItemResult> {
    return this.mutate("delete", (api) => api.remove(id));
  }

  private async mutate(action: ItemAction, run: (api: ReturnType<Session["api"]>) => Promise<unknown>): Promise<ItemResult> {
    const tokens = this.tokens;
    const active = this.state.active;
    if (!tokens || !active?.project) return { ok: false, message: "Pilih project lebih dulu." };
    try {
      await run(this.api(tokens, active));
    } catch (error) {
      if (isEnded(error)) return { ok: false, message: "Sesi berakhir." };
      if (error instanceof ItemsApiError && error.status === 404) await this.loadItems();
      return { ok: false, message: describeItemsError(error, action) };
    }
    await this.loadItems();
    return { ok: true };
  }

  private api(tokens: TokenManager, active: ActiveContext) {
    return itemsApi(tokens, this.config.inventarisUrl, { org: active.org, service: SERVICE, project: active.project });
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

// A session that ended already shows "Sesi berakhir"; its errors need no other message.
function isEnded(error: unknown): boolean {
  return error instanceof TokenError && error.kind === "session_ended";
}

function switchReason(error: unknown): string {
  if (error instanceof TokenError && error.code === "access_denied") {
    return "NICE menolak: akun Anda tidak boleh memakai inventaris di organisasi itu (access_denied).";
  }
  if (error instanceof TokenError && error.code === "invalid_scope") {
    return "NICE menolak: project itu tidak ada, tidak mengaktifkan inventaris, atau Anda tidak punya akses ke sana (invalid_scope).";
  }
  return describe(error);
}

function describe(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
