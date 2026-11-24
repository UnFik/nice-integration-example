import { DEVICE_STATUSES, type Device, type DeviceInput } from "./devices";

// Every call goes to the origin of the page: the backend holds the NICE
// tokens and the session cookie stands for them, so the UI never sees a token.

export interface User {
  id: string;
  name: string;
  email: string;
}

export interface ActiveContext {
  org: string;
  project: string;
}

export interface SessionInfo {
  user: User;
  context?: ActiveContext;
  perms: string[];
  csrfToken: string;
  contextError?: string;
}

export interface ContextProject {
  id: string;
  name: string;
  slug: string;
}

export interface ContextOrganization {
  id: string;
  name: string;
  slug: string;
  actAs: boolean;
  projects: ContextProject[];
  projectsTruncated: boolean;
}

export interface Contexts {
  currentOrganizationId: string;
  organizations: ContextOrganization[];
}

// ApiError carries the {"error","error_description"} body of the backend.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code?: string,
    readonly description?: string,
  ) {
    super(`Backend menjawab HTTP ${status}${code ? ` ${code}` : ""}${description ? ` (${description})` : ""}.`);
    this.name = "ApiError";
  }
}

export type Fetch = typeof fetch;

export type Api = ReturnType<typeof createApi>;

// createApi takes the CSRF token as a getter, so a token renewed by a fresh
// GET /auth/session is sent by the next write without building a new client.
export function createApi(fetcher: Fetch, csrfToken: () => string | undefined) {
  const send = async (path: string, method: string, body?: unknown): Promise<unknown> => {
    const headers: Record<string, string> = { Accept: "application/json" };
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (method !== "GET") {
      const csrf = csrfToken();
      if (csrf) headers["X-CSRF-Token"] = csrf;
    }
    const response = await fetcher(path, {
      method,
      headers,
      credentials: "same-origin",
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
    });
    if (response.status === 204) return undefined;
    const answer = (await response.json().catch(() => undefined)) as Record<string, unknown> | undefined;
    if (!response.ok) {
      throw new ApiError(
        response.status,
        typeof answer?.error === "string" ? answer.error : undefined,
        typeof answer?.error_description === "string" && answer.error_description ? answer.error_description : undefined,
      );
    }
    return answer;
  };
  const devicePath = (id: string) => `/devices/${encodeURIComponent(id)}`;

  return {
    async session(): Promise<SessionInfo> {
      return toSessionInfo(await send("/auth/session", "GET"));
    },
    async contexts(): Promise<Contexts> {
      return toContexts(await send("/auth/contexts", "GET"));
    },
    async switchContext(target: ActiveContext): Promise<SessionInfo> {
      return toSessionInfo(await send("/auth/context", "POST", { org: target.org, project: target.project }));
    },
    async logout(): Promise<void> {
      await send("/auth/logout", "POST");
    },
    async listDevices(): Promise<Device[]> {
      const answer = (await send("/devices", "GET")) as { devices?: unknown } | undefined;
      if (!Array.isArray(answer?.devices)) throw new ApiError(200, "invalid_body");
      return answer.devices.map(toDevice);
    },
    async createDevice(input: DeviceInput): Promise<Device> {
      return toDevice(await send("/devices", "POST", toBody(input)));
    },
    async updateDevice(id: string, input: DeviceInput): Promise<Device> {
      return toDevice(await send(devicePath(id), "PUT", toBody(input)));
    },
    async removeDevice(id: string): Promise<void> {
      await send(devicePath(id), "DELETE");
    },
  };
}

// loginUrl is a page navigation, not a fetch: the backend answers with a
// redirect to NICE, which a fetch could not follow across origins.
export function loginUrl(options: { selectAccount?: boolean } = {}): string {
  return options.selectAccount ? "/auth/login?prompt=select_account" : "/auth/login";
}

function toBody(input: DeviceInput): Record<string, string> {
  return { name: input.name, serial_number: input.serialNumber, status: input.status, location: input.location };
}

export function toDevice(raw: unknown): Device {
  const value = record(raw);
  return {
    id: text(value.id),
    name: text(value.name),
    serialNumber: text(value.serial_number),
    status: DEVICE_STATUSES.find((status) => status === value.status) ?? "inactive",
    location: text(value.location),
    createdBy: text(value.created_by),
    actedBy: text(value.acted_by) || undefined,
    createTime: text(value.create_time) || undefined,
    updateTime: text(value.update_time) || undefined,
  };
}

function toSessionInfo(raw: unknown): SessionInfo {
  const value = record(raw);
  const user = record(value.user);
  const context = value.context ? record(value.context) : undefined;
  const csrfToken = text(value.csrf_token);
  if (!csrfToken) throw new ApiError(200, "invalid_body", "csrf_token");
  return {
    user: { id: text(user.id), name: text(user.name), email: text(user.email) },
    context: context && text(context.org) && text(context.project) ? { org: text(context.org), project: text(context.project) } : undefined,
    perms: Array.isArray(value.perms) ? value.perms.filter((perm): perm is string => typeof perm === "string") : [],
    csrfToken,
    contextError: text(value.context_error) || undefined,
  };
}

function toContexts(raw: unknown): Contexts {
  const value = record(raw);
  if (!Array.isArray(value.organizations)) throw new ApiError(200, "invalid_body", "organizations");
  return {
    currentOrganizationId: text(value.current_organization_id),
    organizations: value.organizations.map((entry: unknown) => {
      const org = record(entry);
      const projects = Array.isArray(org.projects) ? org.projects : [];
      return {
        id: text(org.id),
        name: text(org.name),
        slug: text(org.slug),
        actAs: org.act_as === true,
        projects: projects.map((device: unknown) => {
          const project = record(device);
          return { id: text(project.id), name: text(project.name), slug: text(project.slug) };
        }),
        projectsTruncated: org.projects_truncated === true,
      };
    }),
  };
}

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" ? (value as Record<string, unknown>) : {};
}

function text(value: unknown): string {
  return typeof value === "string" ? value : "";
}
