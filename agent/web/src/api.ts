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

// ApiError carries the error body of the backend: /auth/* answers
// {"error","error_description"}, /chat/* answers {"error","message"}.
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
  const call = async (path: string, method: string, body: unknown, accept: string): Promise<Response> => {
    const headers: Record<string, string> = { Accept: accept };
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
    if (!response.ok) throw await errorOf(response);
    return response;
  };

  const send = async (path: string, method: string, body?: unknown): Promise<unknown> => {
    const response = await call(path, method, body, "application/json");
    if (response.status === 204) return undefined;
    return response.json().catch(() => undefined);
  };

  return {
    send,
    // open starts a POST answered with server-sent events. An error before the
    // stream starts is an ordinary JSON answer and is thrown as ApiError.
    async open(path: string, body: unknown): Promise<Response> {
      const response = await call(path, "POST", body, "text/event-stream");
      if (!response.body) throw new ApiError(response.status, "invalid_body", "empty stream");
      return response;
    },
    async session(): Promise<SessionInfo> {
      return toSessionInfo(await send("/auth/session", "GET"));
    },
    async contexts(): Promise<Contexts> {
      return toContexts(await send("/auth/contexts", "GET"));
    },
    async switchContext(target: ActiveContext): Promise<SessionInfo> {
      return toSessionInfo(await send("/auth/context", "POST", { org: target.org, project: target.project }));
    },
    // logout ends the session of the agent and answers where the browser ends its NICE session too.
    async logout(): Promise<{ endSessionUrl?: string }> {
      const answer = (await send("/auth/logout", "POST")) as { end_session_url?: unknown } | undefined;
      return { endSessionUrl: navigable(answer?.end_session_url) };
    },
  };
}

async function errorOf(response: Response): Promise<ApiError> {
  const answer = (await response.json().catch(() => undefined)) as Record<string, unknown> | undefined;
  const description = text(answer?.error_description) || text(answer?.message);
  return new ApiError(response.status, text(answer?.error) || undefined, description || undefined);
}

// navigable keeps only an http(s) address, so the answer cannot send the page to a javascript: URL.
function navigable(value: unknown): string | undefined {
  if (typeof value !== "string" || !value) return undefined;
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:" ? url.href : undefined;
  } catch {
    return undefined;
  }
}

// loginUrl is a page navigation, not a fetch: the backend answers with a
// redirect to NICE, which a fetch could not follow across origins.
export function loginUrl(options: { selectAccount?: boolean } = {}): string {
  return options.selectAccount ? "/auth/login?prompt=select_account" : "/auth/login";
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
        projects: projects.map((item: unknown) => {
          const project = record(item);
          return { id: text(project.id), name: text(project.name), slug: text(project.slug) };
        }),
        projectsTruncated: org.projects_truncated === true,
      };
    }),
  };
}

export function record(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

export function text(value: unknown): string {
  return typeof value === "string" ? value : "";
}
