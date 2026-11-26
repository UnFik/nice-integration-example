// FakeBackend answers /auth/* and /chat/* like the agent backend, so the UI is
// tested without a backend, NICE, or an LLM running.

export const ORG_A = "org_00000000-0000-7000-8000-00000000000a";
export const ORG_B = "org_00000000-0000-7000-8000-00000000000b";
export const PRJ_P = "prj_00000000-0000-7000-8000-000000000001";
export const PRJ_R = "prj_00000000-0000-7000-8000-000000000003";

export interface Recorded {
  method: string;
  url: string;
  headers: Headers;
  credentials?: RequestCredentials;
  body?: unknown;
}

export function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

// sse answers a turn: each chunk is sent as it is, so a test can cut frames anywhere.
export function sse(chunks: string[]): Response {
  const encoder = new TextEncoder();
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
      controller.close();
    },
  });
  return new Response(body, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

export function frame(type: string, data: unknown): string {
  return `event: ${type}\ndata: ${JSON.stringify(data)}\n\n`;
}

export interface StoredConversation {
  id: string;
  title: string;
  created_at: string;
  updated_at: string;
  messages: unknown[];
}

export function conversation(id: string, title: string, updated: string, messages: unknown[] = []): StoredConversation {
  return { id, title, created_at: "2026-11-01T10:00:00Z", updated_at: updated, messages };
}

export class FakeBackend {
  signedIn = true;
  ended = false;
  context: { org: string; project: string } | null = { org: ORG_A, project: PRJ_P };
  csrf = "csrf-1";
  chatEnabled = true;
  // conversations are those of the signed-in user: the backend keys them by sub.
  conversations: StoredConversation[] = [
    conversation("c2", "Daftar perangkat", "2026-11-02T10:00:00Z"),
    conversation("c1", "", "2026-11-01T10:00:00Z"),
  ];
  // turns are the SSE answers of the next POST .../messages or .../approvals/*.
  turns: Response[] = [];
  // Answers queued for the next call to a path, ahead of the normal behavior.
  next: Record<string, Response[]> = {};
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

  calls(method: string, path: string): Recorded[] {
    return this.requests.filter((request) => request.method === method && request.url.split("?")[0] === path);
  }

  sessionBody() {
    return {
      user: { id: "usr_1", name: "Siti Rahma", email: "siti@nusapala.id" },
      context: this.context,
      perms: this.context ? ["agen.mcp.use"] : [],
      csrf_token: this.csrf,
      context_error: "",
    };
  }

  private route(method: string, path: string, body: Record<string, unknown> | undefined): Response {
    if (method === "GET" && path === "/auth/session") return json(200, this.sessionBody());
    if (method === "GET" && path === "/auth/contexts") {
      return json(200, {
        current_organization_id: ORG_A,
        organizations: [
          { id: ORG_A, name: "Nusapala", slug: "nusapala", projects: [{ id: PRJ_P, name: "Parkir", slug: "parkir" }], projects_truncated: false },
          { id: ORG_B, name: "Bintang Laut", slug: "bintang-laut", projects: [{ id: PRJ_R, name: "Retail", slug: "retail" }], projects_truncated: false },
        ],
      });
    }
    if (method === "POST" && path === "/auth/context") {
      this.context = { org: String(body?.org), project: String(body?.project) };
      return json(200, this.sessionBody());
    }
    if (method === "POST" && path === "/auth/logout") {
      this.signedIn = false;
      return json(200, { end_session_url: "" });
    }
    if (path.startsWith("/chat/")) return this.chatRoute(method, path, body);
    return json(404, { error: "not_found" });
  }

  private chatRoute(method: string, path: string, body: Record<string, unknown> | undefined): Response {
    if (!this.context) return json(409, { error: "context_required" });
    const view = ({ messages: _messages, ...rest }: StoredConversation) => rest;
    if (path === "/chat/conversations") {
      if (method === "GET") {
        const sorted = [...this.conversations].sort((a, b) => b.updated_at.localeCompare(a.updated_at));
        return json(200, { conversations: sorted.map(view), chat_enabled: this.chatEnabled });
      }
      if (method === "POST") {
        const created = conversation(`c${this.nextId++}`, String(body?.title ?? ""), "2026-11-03T10:00:00Z");
        this.conversations.push(created);
        return json(201, { conversation: view(created) });
      }
    }
    const [, , , id, sub, callId] = path.split("/");
    const stored = this.conversations.find((entry) => entry.id === decodeURIComponent(id ?? ""));
    if (!stored) return json(404, { error: "not_found", message: "conversation not found" });
    if (!sub) {
      if (method === "GET") return json(200, { conversation: view(stored), messages: stored.messages });
      if (method === "PATCH") {
        stored.title = String(body?.title);
        return json(200, { conversation: view(stored) });
      }
      if (method === "DELETE") {
        this.conversations = this.conversations.filter((entry) => entry !== stored);
        return new Response(null, { status: 204 });
      }
    }
    if (method === "POST" && (sub === "messages" || (sub === "approvals" && callId))) {
      if (!this.chatEnabled) return json(503, { error: "chat_not_configured", message: "the chat is not configured on this agent" });
      return this.turns.shift() ?? sse([frame("done", { reason: "answered" })]);
    }
    return json(405, { error: "method_not_allowed" });
  }
}
