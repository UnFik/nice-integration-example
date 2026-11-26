import { type ActiveContext, type Api, ApiError, type Contexts, createApi, type Fetch, loginUrl, type SessionInfo, type User } from "./api";
import { type ChatApi, type ChatEvent, type Conversation, createChatApi, describeChatError, type Message, type Thread } from "./chat";

export type Phase = "starting" | "unreachable" | "signed-out" | "login-failed" | "ended" | "signed-in";

export interface RunningTool {
  callId: string;
  name: string;
}

// Turn is the turn this page started and is still reading.
export interface Turn {
  conversationId: string;
  tools: RunningTool[];
}

export interface SessionState {
  phase: Phase;
  message?: string;
  // loginError is the login_error code the backend put in the URL after a failed callback.
  loginError?: string;
  signedOut?: boolean;
  user?: User;
  active?: ActiveContext;
  perms: string[];
  // contextError is why the backend could not get a token for the active context.
  contextError?: string;
  contexts?: Contexts;
  contextsError?: string;
  contextsLoading?: boolean;
  // switching is the context being asked from the backend; active changes only when it agrees.
  switching?: ActiveContext;
  switchError?: string;
  loggingOut?: boolean;
  logoutError?: string;

  conversations?: Conversation[];
  conversationsError?: string;
  conversationsLoading?: boolean;
  // chatEnabled is false when the agent has no LLM: history can be read, nothing sent.
  chatEnabled?: boolean;
  selectedId?: string;
  thread?: Thread;
  threadLoading?: boolean;
  threadError?: string;
  turn?: Turn;
  // approvals holds the catalog message of approval events not yet in the stored history.
  approvals: Record<string, string>;
  // answering is the call whose Setujui/Tolak is being sent.
  answering?: string;
  chatError?: string;
}

export interface SessionDeps {
  fetch: Fetch;
  navigate: (url: string) => void;
  replaceUrl: (path: string) => void;
}

const EMPTY: SessionState = { phase: "starting", perms: [], approvals: {} };

export class Session {
  private state: SessionState = EMPTY;
  private readonly listeners = new Set<() => void>();
  private readonly api: Api;
  private readonly chat: ChatApi;
  private started?: Promise<void>;
  // The CSRF token stays out of the rendered state: only writes need it.
  private csrf?: string;
  // epoch changes whenever the session is dropped, so late answers of the old one are ignored.
  private epoch = 0;
  private localId = 0;
  private lastSignOut: "logout" | "switch" = "logout";

  constructor(private readonly deps: SessionDeps) {
    this.api = createApi(deps.fetch, () => this.csrf);
    this.chat = createChatApi(this.api);
  }

  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  readonly getState = (): SessionState => this.state;

  // start runs once per page load.
  start(href: string): Promise<void> {
    this.started ??= this.run(href);
    return this.started;
  }

  retry(): Promise<void> {
    return this.load();
  }

  // signIn is a page navigation: the backend redirects to NICE and back to /auth/callback.
  signIn(options: { selectAccount?: boolean } = {}): void {
    this.deps.navigate(loginUrl(options));
  }

  async reloadContexts(): Promise<void> {
    if (this.state.phase !== "signed-in") return;
    const epoch = this.epoch;
    this.set({ contextsLoading: true });
    try {
      const contexts = await this.api.contexts();
      if (epoch !== this.epoch) return;
      this.set({ contexts, contextsError: undefined, contextsLoading: false });
    } catch (error) {
      if (epoch !== this.epoch || this.dropped(error)) return;
      this.set({ contextsError: describeContextsError(error), contextsLoading: false });
    }
  }

  // choose asks the backend to move the session to org/project with one
  // refresh at NICE. A refusal keeps the previous context.
  async choose(org: string, project: string): Promise<boolean> {
    if (this.state.phase !== "signed-in" || this.state.switching || this.state.turn) return false;
    const epoch = this.epoch;
    this.set({ switching: { org, project }, switchError: undefined });
    let info: SessionInfo;
    try {
      info = await this.withCsrf(() => this.api.switchContext({ org, project }));
    } catch (error) {
      if (epoch !== this.epoch || this.dropped(error)) return false;
      if (error instanceof ApiError && error.status === 409 && error.code === "context_refused") {
        this.set({
          switching: undefined,
          switchError: `Konteks itu tidak dapat dipakai, Anda tetap di konteks sebelumnya. ${refusalReason(error.description)}`,
        });
        await this.reloadContexts();
      } else {
        this.set({ switching: undefined, switchError: `Pindah konteks gagal, Anda tetap di konteks sebelumnya. ${describe(error)}` });
      }
      return false;
    }
    if (epoch !== this.epoch) return false;
    this.apply(info, { switching: undefined, chatError: undefined });
    await this.loadConversations();
    return true;
  }

  // canChat is whether /chat/* can answer at all: it needs the token of a usable context.
  private canChat(): boolean {
    const { phase, active, contextError } = this.state;
    return phase === "signed-in" && active !== undefined && !contextError;
  }

  async loadConversations(): Promise<void> {
    if (!this.canChat()) return;
    const epoch = this.epoch;
    this.set({ conversationsLoading: true });
    try {
      const { conversations, chatEnabled } = await this.chat.list();
      if (epoch !== this.epoch) return;
      this.set({ conversations, chatEnabled, conversationsError: undefined, conversationsLoading: false });
    } catch (error) {
      if (epoch !== this.epoch || this.dropped(error)) return;
      this.contextAnswer(error);
      this.set({ conversationsError: chatSentence(error), conversationsLoading: false });
      return;
    }
    const selected = this.state.selectedId;
    if (selected && this.state.conversations?.some((entry) => entry.id === selected)) await this.reloadThread();
    else if (selected) this.set({ selectedId: undefined, thread: undefined });
  }

  async select(id: string): Promise<void> {
    if (this.state.selectedId === id && this.state.thread) return;
    this.set({ selectedId: id, thread: undefined, threadError: undefined, chatError: undefined, approvals: {} });
    await this.reloadThread();
  }

  // reloadThread reads the stored history, which is the truth after a turn:
  // the stream may have been cut while the turn kept running on the server.
  async reloadThread(): Promise<void> {
    const id = this.state.selectedId;
    if (!id || !this.canChat()) return;
    const epoch = this.epoch;
    this.set({ threadLoading: true });
    try {
      const thread = await this.chat.get(id);
      if (epoch !== this.epoch || this.state.selectedId !== id) return;
      this.set({ thread, threadError: undefined, threadLoading: false, approvals: {} });
    } catch (error) {
      if (epoch !== this.epoch || this.dropped(error) || this.state.selectedId !== id) return;
      this.contextAnswer(error);
      if (error instanceof ApiError && error.status === 404) {
        this.set({ selectedId: undefined, thread: undefined, threadLoading: false, chatError: describeChatError("not_found") });
        await this.loadConversations();
        return;
      }
      this.set({ threadError: chatSentence(error), threadLoading: false });
    }
  }

  async newConversation(): Promise<boolean> {
    if (!this.canChat()) return false;
    const epoch = this.epoch;
    try {
      const created = await this.withCsrf(() => this.chat.create());
      if (epoch !== this.epoch) return false;
      this.set({
        conversations: [created, ...(this.state.conversations ?? []).filter((entry) => entry.id !== created.id)],
        selectedId: created.id,
        thread: { conversation: created, messages: [] },
        threadError: undefined,
        chatError: undefined,
        approvals: {},
      });
      return true;
    } catch (error) {
      this.chatFailure(error, epoch);
      return false;
    }
  }

  async rename(id: string, title: string): Promise<boolean> {
    const epoch = this.epoch;
    try {
      const renamed = await this.withCsrf(() => this.chat.rename(id, title));
      if (epoch !== this.epoch) return false;
      const thread = this.state.thread;
      this.set({
        conversations: this.state.conversations?.map((entry) => (entry.id === id ? renamed : entry)),
        thread: thread && thread.conversation.id === id ? { ...thread, conversation: renamed } : thread,
        chatError: undefined,
      });
      return true;
    } catch (error) {
      this.chatFailure(error, epoch);
      return false;
    }
  }

  async remove(id: string): Promise<boolean> {
    if (this.state.turn?.conversationId === id) return false;
    const epoch = this.epoch;
    try {
      await this.withCsrf(() => this.chat.remove(id));
    } catch (error) {
      if (!(error instanceof ApiError && error.status === 404)) {
        this.chatFailure(error, epoch);
        return false;
      }
    }
    if (epoch !== this.epoch) return false;
    const selected = this.state.selectedId === id;
    this.set({
      conversations: this.state.conversations?.filter((entry) => entry.id !== id),
      ...(selected ? { selectedId: undefined, thread: undefined, approvals: {} } : {}),
      chatError: undefined,
    });
    return true;
  }

  // send runs one turn. Without a chosen conversation it starts one; the
  // backend names it from the first message.
  async send(message: string): Promise<boolean> {
    const body = message.trim();
    if (!body || this.state.turn || this.state.chatEnabled === false || !this.canChat()) return false;
    if (!this.state.selectedId && !(await this.newConversation())) return false;
    const id = this.state.selectedId as string;
    const local: Message = {
      id: `local-${++this.localId}`,
      seq: Number.MAX_SAFE_INTEGER,
      role: "user",
      createdAt: new Date().toISOString(),
      parts: [{ kind: "text", text: body }],
    };
    this.appendMessage(id, local);
    return this.runTurn(id, () => this.chat.send(id, body, (event) => this.onEvent(id, event)));
  }

  // answer sends Setujui (true) or Tolak (false) for a call that waits for the
  // user. The backend resumes the turn and streams it like a sent message.
  async answer(callId: string, approve: boolean): Promise<boolean> {
    const id = this.state.selectedId;
    if (!id || this.state.turn || this.state.answering) return false;
    this.set({ answering: callId });
    try {
      return await this.runTurn(id, () => this.chat.answer(id, callId, approve, (event) => this.onEvent(id, event)));
    } finally {
      if (this.state.answering === callId) this.set({ answering: undefined });
    }
  }

  private async runTurn(id: string, run: () => Promise<boolean>): Promise<boolean> {
    const epoch = this.epoch;
    this.set({ turn: { conversationId: id, tools: [] }, chatError: undefined });
    let ok = true;
    try {
      const ended = await this.withCsrf(run);
      if (epoch !== this.epoch) return false;
      if (!ended) this.set({ chatError: "Sambungan ke agen terputus sebelum giliran selesai. Giliran tetap berjalan di backend; riwayat dimuat ulang." });
    } catch (error) {
      if (epoch !== this.epoch || this.dropped(error)) return false;
      ok = false;
      this.contextAnswer(error);
      if (error instanceof ApiError && error.code === "chat_not_configured") this.set({ chatEnabled: false });
      this.set({ chatError: chatSentence(error) });
    }
    if (epoch !== this.epoch) return false;
    this.set({ turn: undefined });
    await Promise.all([this.reloadThread(), this.refreshList()]);
    return ok;
  }

  private onEvent(id: string, event: ChatEvent): void {
    const turn = this.state.turn;
    if (!turn || turn.conversationId !== id) return;
    switch (event.type) {
      case "assistant":
      case "tool_result":
        this.appendMessage(id, event.message);
        if (event.type === "tool_result") {
          const done = new Set(event.message.parts.flatMap((part) => (part.kind === "tool_result" ? [part.result.callId] : [])));
          this.set({ turn: { ...turn, tools: turn.tools.filter((tool) => !done.has(tool.callId)) } });
        }
        return;
      case "tool_call":
        this.set({ turn: { ...turn, tools: [...turn.tools, { callId: event.callId, name: event.name }] } });
        return;
      case "approval":
        this.set({
          approvals: { ...this.state.approvals, [event.callId]: event.message },
          turn: { ...turn, tools: turn.tools.filter((tool) => tool.callId !== event.callId) },
        });
        return;
      case "error":
        if (event.code === "chat_not_configured") this.set({ chatEnabled: false });
        this.set({ chatError: describeChatError(event.code, event.message) });
        return;
      case "done":
        if (event.reason === "step_limit") this.set({ chatError: "Giliran berhenti karena batas langkah tool tercapai." });
        return;
    }
  }

  private appendMessage(id: string, message: Message): void {
    const thread = this.state.thread;
    if (!thread || thread.conversation.id !== id || this.state.selectedId !== id) return;
    this.set({ thread: { ...thread, messages: [...thread.messages, message] } });
  }

  // refreshList keeps the order of the list (newest change first) after a turn.
  private async refreshList(): Promise<void> {
    if (!this.canChat()) return;
    const epoch = this.epoch;
    try {
      const { conversations, chatEnabled } = await this.chat.list();
      if (epoch === this.epoch) this.set({ conversations, chatEnabled });
    } catch (error) {
      if (epoch === this.epoch) this.dropped(error);
    }
  }

  private chatFailure(error: unknown, epoch: number): void {
    if (epoch !== this.epoch || this.dropped(error)) return;
    this.contextAnswer(error);
    this.set({ chatError: chatSentence(error) });
  }

  // logout signs out of the agent and then of NICE, so the next sign-in asks
  // for the password. Without an end-session address it stops at the signed-out screen.
  async logout(): Promise<void> {
    this.lastSignOut = "logout";
    const endSessionUrl = await this.signOut();
    if (endSessionUrl) this.deps.navigate(endSessionUrl);
  }

  // switchAccount signs out of the agent only: NICE keeps its session and shows
  // its account chooser, so the user can pick or add another account.
  async switchAccount(): Promise<void> {
    this.lastSignOut = "switch";
    if ((await this.signOut()) !== false) this.deps.navigate(loginUrl({ selectAccount: true }));
  }

  retrySignOut(): Promise<void> {
    return this.lastSignOut === "switch" ? this.switchAccount() : this.logout();
  }

  // signOut keeps the session when the backend could not revoke the refresh
  // token at NICE, so the user never looks signed out while it still works.
  // It answers false when the session was kept, else the end-session address.
  private async signOut(): Promise<string | undefined | false> {
    if (this.state.phase !== "signed-in" || this.state.loggingOut) return false;
    const epoch = this.epoch;
    this.set({ loggingOut: true, logoutError: undefined });
    let endSessionUrl: string | undefined;
    try {
      ({ endSessionUrl } = await this.withCsrf(() => this.api.logout()));
    } catch (error) {
      if (epoch !== this.epoch || this.dropped(error)) return false;
      const message =
        error instanceof ApiError && error.status === 503
          ? "NICE tidak menjawab, jadi refresh token belum dicabut dan Anda belum keluar. Sesi tetap aktif."
          : `Keluar gagal, sesi tetap aktif. ${describe(error)}`;
      this.set({ loggingOut: false, logoutError: message });
      return false;
    }
    this.reset({ phase: "signed-out", signedOut: true });
    return endSessionUrl;
  }

  private async run(href: string): Promise<void> {
    const url = new URL(href);
    const loginError = url.searchParams.get("login_error");
    if (loginError !== null) {
      // The code leaves the address bar so a reload does not show the failure again.
      url.searchParams.delete("login_error");
      this.deps.replaceUrl(`${url.pathname}${url.search}${url.hash}`);
      this.reset({ phase: "login-failed", loginError: loginError || "unknown" });
      return;
    }
    await this.load();
  }

  private async load(): Promise<void> {
    this.reset({ phase: "starting" });
    const epoch = this.epoch;
    let info: SessionInfo;
    try {
      info = await this.api.session();
    } catch (error) {
      if (epoch !== this.epoch || this.dropped(error)) return;
      this.reset({ phase: "unreachable", message: `GET /auth/session gagal: ${describe(error)}` });
      return;
    }
    if (epoch !== this.epoch) return;
    this.apply(info, { phase: "signed-in" });
    await Promise.all([this.reloadContexts(), this.loadConversations()]);
  }

  // withCsrf retries a write once after 403 csrf with the token of a fresh
  // GET /auth/session: the backend may have renewed it meanwhile.
  private async withCsrf<T>(run: () => Promise<T>): Promise<T> {
    try {
      return await run();
    } catch (error) {
      if (!(error instanceof ApiError && error.status === 403 && error.code === "csrf")) throw error;
      this.csrf = (await this.api.session()).csrfToken;
      return run();
    }
  }

  // contextAnswer follows a 409 of /chat/*: the backend has no usable token
  // for the active context, so the picker takes over.
  private contextAnswer(error: unknown): void {
    if (!(error instanceof ApiError && error.status === 409)) return;
    if (error.code === "context_required") this.set({ active: undefined, perms: [] });
    if (error.code === "context_refused") this.set({ contextError: error.description || "access_denied", perms: [] });
  }

  // dropped ends the session on 401: session_ended means NICE ended it.
  private dropped(error: unknown): boolean {
    if (!(error instanceof ApiError && error.status === 401)) return false;
    this.reset(error.code === "session_ended" ? { phase: "ended" } : { phase: "signed-out" });
    return true;
  }

  private apply(info: SessionInfo, patch: Partial<SessionState>): void {
    this.csrf = info.csrfToken;
    this.set({ ...patch, user: info.user, active: info.context, perms: info.perms, contextError: info.contextError });
  }

  private reset(patch: Partial<SessionState>): void {
    this.epoch++;
    this.csrf = undefined;
    this.state = { ...EMPTY, ...patch };
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

function chatSentence(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 409 && (error.code === "context_required" || error.code === "context_refused")) {
      return "Konteks aktif tidak dapat dipakai untuk chat. Pilih organisasi dan project lain.";
    }
    if (error.status === 503 && error.code === "temporarily_unavailable") return "NICE tidak menjawab. Coba lagi sebentar lagi.";
    return describeChatError(error.code ?? `HTTP ${error.status}`, error.description);
  }
  return describeChatError(undefined, describe(error));
}

// refusalReason explains the NICE code behind a refused context token.
export function refusalReason(code: string | undefined): string {
  switch (code) {
    case "access_denied":
      return "NICE menolak: akun Anda tidak boleh memakai agen di organisasi itu (access_denied).";
    case "invalid_scope":
      return "NICE menolak: project itu tidak ada, tidak mengaktifkan agen, atau Anda tidak punya akses ke sana (invalid_scope).";
    case "temporarily_unavailable":
      return "NICE tidak menjawab. Coba lagi sebentar lagi (temporarily_unavailable).";
    default:
      return `NICE menolak${code ? ` (${code})` : ""}.`;
  }
}

export function describeLoginError(code: string): { title: string; message: string; selectAccount: boolean } {
  switch (code) {
    case "access_denied":
      return {
        title: "Agen tidak tersedia",
        message:
          "Tidak ada organisasi Anda yang memakai agen, jadi NICE menolak login ke service ini. Minta admin organisasi mengaktifkan agen, atau masuk dengan akun lain.",
        selectAccount: true,
      };
    case "state_mismatch":
      return {
        title: "Login gagal",
        message: "Jawaban login tidak cocok dengan sesi browser ini, mis. karena login dimulai di tab lain atau tautannya sudah dipakai. Masuk lagi.",
        selectAccount: false,
      };
    case "temporarily_unavailable":
      return { title: "NICE tidak menjawab", message: "Login belum selesai karena NICE tidak menjawab. Coba masuk lagi sebentar lagi.", selectAccount: false };
    default:
      return { title: "Login gagal", message: `Login tidak selesai (${code}). Masuk lagi.`, selectAccount: false };
  }
}

function describeContextsError(error: unknown): string {
  const cause =
    error instanceof ApiError && error.code === "contexts_unavailable"
      ? `NICE menolak penemuan konteks${error.description ? ` (${error.description})` : ""}.`
      : error instanceof ApiError && error.status === 503
        ? "NICE tidak menjawab (temporarily_unavailable)."
        : describe(error);
  return `Pilihan konteks tidak dapat dimuat: ${cause} Konteks aktif tetap dipakai.`;
}

function describe(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
