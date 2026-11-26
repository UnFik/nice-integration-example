import { beforeEach, describe, expect, it } from "vitest";
import { Session } from "./session";
import { conversation, FakeBackend, frame, ORG_B, PRJ_R, sse } from "./test/fake-backend";

let backend: FakeBackend;
let navigations: string[];

beforeEach(() => {
  backend = new FakeBackend();
  navigations = [];
});

async function started(href = "http://localhost:5182/"): Promise<Session> {
  const session = new Session({ fetch: backend.fetch, navigate: (url) => navigations.push(url), replaceUrl: () => {} });
  await session.start(href);
  return session;
}

const awaiting = [
  { id: "m1", seq: 1, role: "user", created_at: "2026-11-02T10:00:00Z", parts: [{ kind: "text", text: "hapus dev_1" }] },
  {
    id: "m2",
    seq: 2,
    role: "assistant",
    created_at: "2026-11-02T10:00:01Z",
    parts: [
      {
        kind: "tool_call",
        call: { id: "call_1", name: "call_operation", input: { service: "nadi", operation: "deleteDevice" }, status: "awaiting_approval", approval_message: "Hapus perangkat dev_1 di nadi?" },
      },
    ],
  },
];

describe("session", () => {
  it("shows Masuk without a session and navigates to /auth/login", async () => {
    backend.signedIn = false;
    const session = await started();

    expect(session.getState().phase).toBe("signed-out");
    session.signIn();
    expect(navigations).toEqual(["/auth/login"]);
  });

  it("lists only what the backend answers for this user, newest first", async () => {
    const state = (await started()).getState();

    expect(state.phase).toBe("signed-in");
    expect(state.conversations?.map((entry) => entry.id)).toEqual(["c2", "c1"]);
    expect(state.chatEnabled).toBe(true);
    expect(backend.requests.every((request) => request.url.startsWith("/auth/") || request.url.startsWith("/chat/"))).toBe(true);
  });

  it("does not ask for conversations without an active context", async () => {
    backend.context = null;
    const state = (await started()).getState();

    expect(state.conversations).toBeUndefined();
    expect(backend.calls("GET", "/chat/conversations")).toHaveLength(0);
  });

  it("reloads the list after a context switch", async () => {
    const session = await started();
    await session.choose(ORG_B, PRJ_R);

    expect(session.getState().active).toEqual({ org: ORG_B, project: PRJ_R });
    expect(backend.calls("GET", "/chat/conversations")).toHaveLength(2);
  });

  it("starts a conversation on the first message and reloads the stored history after the turn", async () => {
    const session = await started();
    backend.turns.push(sse([frame("assistant", { message: { id: "m2", role: "assistant", parts: [{ kind: "text", text: "Ada 1 perangkat." }] } }), frame("done", { reason: "answered" })]));

    const sending = session.send("daftar perangkat");
    expect(session.getState().turn).toBeUndefined();
    await sending;

    const created = backend.calls("POST", "/chat/conversations");
    expect(created).toHaveLength(1);
    const id = session.getState().selectedId as string;
    expect(backend.calls("POST", `/chat/conversations/${id}/messages`)[0].body).toEqual({ text: "daftar perangkat" });
    expect(backend.calls("GET", `/chat/conversations/${id}`)).toHaveLength(1);
    expect(session.getState().turn).toBeUndefined();
  });

  it("keeps the turn open while the stream runs, so the input is disabled", async () => {
    const session = await started();
    await session.select("c1");
    let release!: () => void;
    const gate = new Promise<void>((resolve) => (release = resolve));
    const encoder = new TextEncoder();
    backend.turns.push(
      new Response(
        new ReadableStream<Uint8Array>({
          async start(controller) {
            controller.enqueue(encoder.encode(frame("tool_call", { call_id: "call_9", name: "list_services" })));
            await gate;
            controller.enqueue(encoder.encode(frame("done", { reason: "answered" })));
            controller.close();
          },
        }),
        { status: 200 },
      ),
    );

    const sending = session.send("halo");
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(session.getState().turn).toEqual({ conversationId: "c1", tools: [{ callId: "call_9", name: "list_services" }] });
    expect(await session.send("lagi")).toBe(false);
    release();
    await sending;
    expect(session.getState().turn).toBeUndefined();
  });

  it("approves with POST .../approvals/{call_id}, {approve:true}, and the CSRF header", async () => {
    backend.conversations = [conversation("c1", "Hapus", "2026-11-02T10:00:00Z", awaiting)];
    const session = await started();
    await session.select("c1");
    expect(session.getState().thread?.messages).toHaveLength(2);

    await session.answer("call_1", true);

    const [request] = backend.calls("POST", "/chat/conversations/c1/approvals/call_1");
    expect(request.body).toEqual({ approve: true });
    expect(request.headers.get("X-CSRF-Token")).toBe("csrf-1");
    expect(session.getState().answering).toBeUndefined();
  });

  it("declines with {approve:false}", async () => {
    backend.conversations = [conversation("c1", "Hapus", "2026-11-02T10:00:00Z", awaiting)];
    const session = await started();
    await session.select("c1");

    await session.answer("call_1", false);

    expect(backend.calls("POST", "/chat/conversations/c1/approvals/call_1")[0].body).toEqual({ approve: false });
  });

  it("shows turn_in_progress and keeps the draft path open", async () => {
    const session = await started();
    await session.select("c1");
    backend.next["/chat/conversations/c1/messages"] = [new Response(JSON.stringify({ error: "turn_in_progress", message: "busy" }), { status: 409 })];

    expect(await session.send("halo")).toBe(false);
    expect(session.getState().chatError).toMatch(/masih berjalan/);
  });

  it("turns off the input after 503 chat_not_configured", async () => {
    const session = await started();
    await session.select("c1");
    backend.chatEnabled = false;

    await session.send("halo");
    expect(session.getState().chatEnabled).toBe(false);
    expect(session.getState().chatError).toBe("Chat belum dikonfigurasi di agen.");
  });

  it("shows an error event of the turn", async () => {
    const session = await started();
    await session.select("c1");
    backend.turns.push(sse([frame("error", { error: "model_unavailable", message: "later" })]));

    await session.send("halo");
    expect(session.getState().chatError).toMatch(/Model LLM sedang tidak tersedia/);
  });

  it("renames and deletes a conversation", async () => {
    const session = await started();
    await session.select("c1");

    expect(await session.rename("c1", "Gudang")).toBe(true);
    expect(session.getState().conversations?.find((entry) => entry.id === "c1")?.title).toBe("Gudang");
    expect(await session.remove("c1")).toBe(true);
    expect(session.getState().conversations?.map((entry) => entry.id)).toEqual(["c2"]);
    expect(session.getState().selectedId).toBeUndefined();
  });

  it("retries a write once with a renewed CSRF token", async () => {
    const session = await started();
    backend.csrf = "csrf-2";

    expect(await session.rename("c1", "Baru")).toBe(true);
    expect(backend.calls("PATCH", "/chat/conversations/c1").map((request) => request.headers.get("X-CSRF-Token"))).toEqual(["csrf-1", "csrf-2"]);
  });

  it("drops to Sesi berakhir on 401 session_ended", async () => {
    const session = await started();
    backend.ended = true;

    await session.select("c1");
    expect(session.getState().phase).toBe("ended");
  });
});
