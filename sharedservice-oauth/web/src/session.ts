import { type ActiveContext, type Api, ApiError, type Contexts, createApi, type Fetch, loginUrl, type SessionInfo, type User } from "./api";
import { describeDevicesError, DEVICE_PERMISSIONS, type Device, type DeviceAction, type DeviceInput } from "./devices";

export type Phase = "starting" | "unreachable" | "signed-out" | "login-failed" | "ended" | "signed-in";

export type { Device } from "./devices";

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
  devices?: Device[];
  devicesError?: string;
  devicesLoading?: boolean;
  loggingOut?: boolean;
  logoutError?: string;
}

export type DeviceResult = { ok: true } | { ok: false; message: string };

export interface SessionDeps {
  fetch: Fetch;
  navigate: (url: string) => void;
  replaceUrl: (path: string) => void;
}

const EMPTY: SessionState = { phase: "starting", perms: [] };

export class Session {
  private state: SessionState = EMPTY;
  private readonly listeners = new Set<() => void>();
  private readonly api: Api;
  private started?: Promise<void>;
  // The CSRF token stays out of the rendered state: only writes need it.
  private csrf?: string;
  // epoch changes whenever the session is dropped, so late answers of the old one are ignored.
  private epoch = 0;

  constructor(private readonly deps: SessionDeps) {
    this.api = createApi(deps.fetch, () => this.csrf);
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
    if (this.state.phase !== "signed-in" || this.state.switching) return false;
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
        // The choices read earlier are stale: the access changed since.
        await this.reloadContexts();
      } else {
        this.set({ switching: undefined, switchError: `Pindah konteks gagal, Anda tetap di konteks sebelumnya. ${describe(error)}` });
      }
      return false;
    }
    if (epoch !== this.epoch) return false;
    this.apply(info, { switching: undefined, devices: undefined, devicesError: undefined });
    await this.loadDevices();
    return true;
  }

  async loadDevices(): Promise<void> {
    const { active, contextError, perms, phase } = this.state;
    if (phase !== "signed-in" || !active || contextError) return;
    if (!perms.includes(DEVICE_PERMISSIONS.read)) {
      this.set({ devices: undefined, devicesLoading: false, devicesError: `Token konteks ini tidak memuat ${DEVICE_PERMISSIONS.read}, jadi daftar perangkat tidak dapat dibaca.` });
      return;
    }
    const epoch = this.epoch;
    this.set({ devicesLoading: true });
    try {
      const devices = await this.api.listDevices();
      if (epoch !== this.epoch || this.state.active !== active) return;
      this.set({ devices, devicesError: undefined, devicesLoading: false });
    } catch (error) {
      if (epoch !== this.epoch || this.dropped(error) || this.state.active !== active) return;
      this.contextAnswer(error);
      this.set({ devices: undefined, devicesError: describeDevicesError(error, "read"), devicesLoading: false });
    }
  }

  // saveDevice creates a device, or changes the one with id, then reloads the list.
  // The answer is a sentence for the user; the list is only touched on success.
  saveDevice(input: DeviceInput, id?: string): Promise<DeviceResult> {
    return this.mutate(id ? "edit" : "create", () => (id ? this.api.updateDevice(id, input) : this.api.createDevice(input)));
  }

  removeDevice(id: string): Promise<DeviceResult> {
    return this.mutate("delete", () => this.api.removeDevice(id));
  }

  // logout keeps the session when the backend could not revoke the refresh
  // token at NICE, so the user never looks signed out while it still works.
  async logout(): Promise<void> {
    if (this.state.phase !== "signed-in" || this.state.loggingOut) return;
    const epoch = this.epoch;
    this.set({ loggingOut: true, logoutError: undefined });
    try {
      await this.withCsrf(() => this.api.logout());
    } catch (error) {
      if (epoch !== this.epoch || this.dropped(error)) return;
      const message =
        error instanceof ApiError && error.status === 503
          ? "NICE tidak menjawab, jadi refresh token belum dicabut dan Anda belum keluar. Sesi tetap aktif."
          : `Keluar gagal, sesi tetap aktif. ${describe(error)}`;
      this.set({ loggingOut: false, logoutError: message });
      return;
    }
    this.reset({ phase: "signed-out", signedOut: true });
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
    await Promise.all([this.reloadContexts(), this.loadDevices()]);
  }

  private async mutate(action: DeviceAction, run: () => Promise<unknown>): Promise<DeviceResult> {
    if (!this.state.active) return { ok: false, message: "Pilih project lebih dulu." };
    const epoch = this.epoch;
    try {
      await this.withCsrf(run);
    } catch (error) {
      if (epoch !== this.epoch || this.dropped(error)) return { ok: false, message: "Sesi berakhir." };
      this.contextAnswer(error);
      if (error instanceof ApiError && error.status === 404) await this.loadDevices();
      return { ok: false, message: describeDevicesError(error, action) };
    }
    await this.loadDevices();
    return { ok: true };
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

  // contextAnswer follows a 409 of a device operation: the backend has no usable
  // token for the active context, so the picker takes over.
  private contextAnswer(error: unknown): void {
    if (!(error instanceof ApiError && error.status === 409)) return;
    if (error.code === "context_required") this.set({ active: undefined, perms: [], devices: undefined });
    if (error.code === "context_refused") this.set({ contextError: error.description || "access_denied", perms: [], devices: undefined });
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

// refusalReason explains the NICE code behind a refused context token.
export function refusalReason(code: string | undefined): string {
  switch (code) {
    case "access_denied":
      return "NICE menolak: akun Anda tidak boleh memakai nadi di organisasi itu (access_denied).";
    case "invalid_scope":
      return "NICE menolak: project itu tidak ada, tidak mengaktifkan nadi, atau Anda tidak punya akses ke sana (invalid_scope).";
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
        title: "Service tidak tersedia",
        message:
          "Tidak ada organisasi Anda yang memakai nadi, jadi NICE menolak login ke service ini. Minta admin organisasi mengaktifkan nadi, atau masuk dengan akun lain.",
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
