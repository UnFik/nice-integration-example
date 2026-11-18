// FakeBackend answers /auth/* and /devices* like the nadi backend, so the
// UI is tested without a backend or NICE running.

export const ORG_A = "org_00000000-0000-7000-8000-00000000000a";
export const ORG_B = "org_00000000-0000-7000-8000-00000000000b";
export const ORG_C = "org_00000000-0000-7000-8000-00000000000c";
export const PRJ_P = "prj_00000000-0000-7000-8000-000000000001";
export const PRJ_Q = "prj_00000000-0000-7000-8000-000000000002";
export const PRJ_R = "prj_00000000-0000-7000-8000-000000000003";

export const FULL = ["nadi.devices.read", "nadi.devices.create", "nadi.devices.edit", "nadi.devices.delete"];

export interface Recorded {
  method: string;
  url: string;
  headers: Headers;
  credentials?: RequestCredentials;
  body?: unknown;
}

interface StoredDevice {
  id: string;
  name: string;
  serial_number: string;
  status: string;
  location: string;
  created_by: string;
  create_time: string;
  update_time: string;
}

export function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

export class FakeBackend {
  signedIn = true;
  // ended makes every call answer 401 session_ended, as after invalid_grant at NICE.
  ended = false;
  context: { org: string; project: string } | null = { org: ORG_A, project: PRJ_P };
  contextError = "";
  csrf = "csrf-1";
  perms: Record<string, string[]> = { [`${ORG_A}/${PRJ_P}`]: FULL, [`${ORG_A}/${PRJ_Q}`]: ["nadi.devices.read"], [`${ORG_B}/${PRJ_R}`]: FULL };
  // refused maps a context to the NICE code POST /auth/context answers with 409.
  refused: Record<string, string> = {};
  contextsFailure?: Response;
  logoutFailure?: Response;
  devices: Record<string, StoredDevice[]> = {
    [`${ORG_A}/${PRJ_P}`]: [device("dev_1", "Sensor Suhu Gudang", "SN-2026-0001", "active", "Gudang A")],
    [`${ORG_B}/${PRJ_R}`]: [device("dev_9", "Gateway LoRa", "SN-2026-0009", "maintenance", "Atap")],
  };
  // Answers queued for the next call to a path, ahead of the normal behavior.
  next: Record<string, Response[]> = {};
  organizations = [
    {
      id: ORG_A,
      name: "Nusapala",
      slug: "nusapala",
      projects: [
        { id: PRJ_P, name: "Parkir", slug: "parkir" },
        { id: PRJ_Q, name: "Gudang", slug: "gudang" },
      ],
      projects_truncated: false,
    },
    { id: ORG_B, name: "Bintang Laut", slug: "bintang-laut", projects: [{ id: PRJ_R, name: "Retail", slug: "retail" }], projects_truncated: false },
    { id: ORG_C, name: "Cakrawala", slug: "cakrawala", projects: [], projects_truncated: false },
  ];
  readonly requests: Recorded[] = [];
  private nextId = 100;

  readonly fetch = async (input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> => {
    const url = String(input);
    const method = (init.method ?? "GET").toUpperCase();
    const headers = new Headers(init.headers);
    const body = typeof init.body === "string" ? JSON.parse(init.body) : undefined;
    this.requests.push({ method, url, headers, credentials: init.credentials, body });
    if (!url.startsWith("/")) throw new TypeError(`the UI called another origin: ${url}`);
    const path = url.split("?")[0];
    const queued = this.next[path]?.shift();
    if (queued) return queued;
    if (this.ended) return json(401, { error: "session_ended" });
    if (!this.signedIn) return json(401, { error: "unauthenticated" });
    if (method !== "GET" && headers.get("X-CSRF-Token") !== this.csrf) return json(403, { error: "csrf" });
    return this.route(method, path, body);
  };

  get key(): string {
    return this.context ? `${this.context.org}/${this.context.project}` : "";
  }

  sessionBody() {
    return {
      user: { id: "usr_1", name: "Siti Rahma", email: "siti@nusapala.id" },
      context: this.context,
      perms: this.context && !this.contextError ? (this.perms[this.key] ?? []) : [],
      csrf_token: this.csrf,
      context_error: this.contextError,
    };
  }

  calls(method: string, path: string): Recorded[] {
    return this.requests.filter((request) => request.method === method && request.url.split("?")[0] === path);
  }

  private route(method: string, path: string, body: Record<string, unknown> | undefined): Response {
    if (method === "GET" && path === "/auth/session") return json(200, this.sessionBody());
    if (method === "GET" && path === "/auth/contexts") {
      return this.contextsFailure ?? json(200, { current_organization_id: ORG_A, organizations: this.organizations });
    }
    if (method === "POST" && path === "/auth/context") {
      const target = `${body?.org}/${body?.project}`;
      const refusal = this.refused[target];
      if (refusal) return json(409, { error: "context_refused", error_description: refusal });
      this.context = { org: String(body?.org), project: String(body?.project) };
      this.contextError = "";
      return json(200, this.sessionBody());
    }
    if (method === "POST" && path === "/auth/logout") {
      if (this.logoutFailure) return this.logoutFailure;
      this.signedIn = false;
      return new Response(null, { status: 204 });
    }
    if (path === "/devices" || path.startsWith("/devices/")) return this.devicesRoute(method, path, body);
    return json(404, { error: "not_found" });
  }

  private devicesRoute(method: string, path: string, body: Record<string, unknown> | undefined): Response {
    if (!this.context) return json(409, { error: "context_required" });
    if (this.contextError) return json(409, { error: "context_refused", error_description: this.contextError });
    const perms = this.perms[this.key] ?? [];
    const list = (this.devices[this.key] ??= []);
    const need = { GET: "read", POST: "create", PUT: "edit", DELETE: "delete" }[method];
    if (!perms.includes(`nadi.devices.${need}`)) return json(403, { error: "permission_denied" });
    if (method === "GET" && path === "/devices") return json(200, { devices: list });
    if (method === "POST" && path === "/devices") {
      const created = device(`dev_${this.nextId++}`, String(body?.name), String(body?.serial_number), String(body?.status), String(body?.location ?? ""));
      list.push(created);
      return json(201, created);
    }
    const id = decodeURIComponent(path.slice("/devices/".length));
    const index = list.findIndex((entry) => entry.id === id);
    if (index < 0) return json(404, { error: "not_found" });
    if (method === "PUT") {
      list[index] = {
        ...list[index],
        name: String(body?.name),
        serial_number: String(body?.serial_number),
        status: String(body?.status),
        location: String(body?.location ?? ""),
      };
      return json(200, list[index]);
    }
    if (method === "DELETE") {
      list.splice(index, 1);
      return new Response(null, { status: 204 });
    }
    return json(405, { error: "method_not_allowed" });
  }
}

function device(id: string, name: string, serial_number: string, status: string, location: string): StoredDevice {
  return { id, name, serial_number, status, location, created_by: "usr_1", create_time: "2026-11-01T10:00:00Z", update_time: "2026-11-02T10:00:00Z" };
}
