import { describe, expect, it } from "vitest";
import { ApiError, createApi } from "./api";
import { type ChatEvent, createChatApi, readEvents } from "./chat";
import { FakeBackend, frame, json, sse } from "./test/fake-backend";

function stream(chunks: string[]): ReadableStream<Uint8Array> {
  return sse(chunks).body as ReadableStream<Uint8Array>;
}

async function events(chunks: string[]): Promise<ChatEvent[]> {
  const seen: ChatEvent[] = [];
  await readEvents(stream(chunks), (event) => seen.push(event));
  return seen;
}

const assistant = {
  message: {
    id: "m2",
    seq: 2,
    role: "assistant",
    created_at: "2026-11-02T10:00:00Z",
    parts: [
      { kind: "text", text: "Saya hapus perangkat itu." },
      { kind: "tool_call", call: { id: "call_1", name: "call_operation", input: { service: "nadi", operation: "deleteDevice" } } },
    ],
  },
};

function client(backend: FakeBackend) {
  return createChatApi(createApi(backend.fetch, () => backend.csrf));
}

describe("readEvents", () => {
  it("joins a frame split across chunks, even inside a multibyte character", async () => {
    const whole = frame("assistant", { message: { ...assistant.message, parts: [{ kind: "text", text: "Perangkat é" }] } });
    const bytes = new TextEncoder().encode(whole);
    const cut = bytes.indexOf(0xc3) + 1;
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(bytes.slice(0, 5));
        controller.enqueue(bytes.slice(5, cut));
        controller.enqueue(bytes.slice(cut));
        controller.close();
      },
    });
    const seen: ChatEvent[] = [];
    await readEvents(body, (event) => seen.push(event));

    expect(seen).toHaveLength(1);
    expect(seen[0]).toMatchObject({ type: "assistant", message: { id: "m2", parts: [{ kind: "text", text: "Perangkat é" }] } });
  });

  it("reads several frames from one chunk and a blank line cut in two", async () => {
    const seen = await events([
      frame("tool_call", { call_id: "call_1", name: "list_services" }) + frame("tool_result", { message: { id: "m3", role: "tool", parts: [{ kind: "tool_result", result: { call_id: "call_1", content: { services: [] } } }] } }) + "event: done\ndata: {\"reason\":",
      "\"answered\"}\n",
      "\n",
    ]);

    expect(seen.map((event) => event.type)).toEqual(["tool_call", "tool_result", "done"]);
    expect(seen[0]).toEqual({ type: "tool_call", callId: "call_1", name: "list_services" });
    expect(seen[1]).toMatchObject({ message: { role: "tool", parts: [{ kind: "tool_result", result: { callId: "call_1", content: { services: [] }, isError: false } }] } });
    expect(seen[2]).toEqual({ type: "done", reason: "answered" });
  });

  it("parses an approval event with CRLF lines and skips comments and unknown events", async () => {
    const seen = await events([
      ": keep-alive\r\n\r\n",
      "event: approval\r\ndata: {\"call_id\":\"call_1\",\"message\":\"Hapus perangkat dev_1 di nadi?\"}\r\n\r\n",
      frame("unknown", { x: 1 }),
      frame("done", { reason: "awaiting_approval" }),
    ]);

    expect(seen).toEqual([
      { type: "approval", callId: "call_1", message: "Hapus perangkat dev_1 di nadi?" },
      { type: "done", reason: "awaiting_approval" },
    ]);
  });

  it("reads an error event", async () => {
    expect(await events([frame("error", { error: "model_unavailable", message: "try again later" })])).toEqual([
      { type: "error", code: "model_unavailable", message: "try again later" },
    ]);
  });
});

describe("chat API", () => {
  it("sends a message with the CSRF header and streams its events", async () => {
    const backend = new FakeBackend();
    backend.turns.push(sse([frame("assistant", assistant), frame("approval", { call_id: "call_1", message: "Hapus?" }), frame("done", { reason: "awaiting_approval" })]));
    const seen: ChatEvent[] = [];

    const ended = await client(backend).send("c1", "hapus dev_1", (event) => seen.push(event));

    expect(ended).toBe(true);
    expect(seen.map((event) => event.type)).toEqual(["assistant", "approval", "done"]);
    const [request] = backend.calls("POST", "/chat/conversations/c1/messages");
    expect(request.body).toEqual({ text: "hapus dev_1" });
    expect(request.headers.get("X-CSRF-Token")).toBe("csrf-1");
    expect(request.headers.get("Accept")).toBe("text/event-stream");
    expect(request.credentials).toBe("same-origin");
  });

  it("says when the stream ended without done or error", async () => {
    const backend = new FakeBackend();
    backend.turns.push(sse([frame("assistant", assistant)]));

    expect(await client(backend).send("c1", "halo", () => {})).toBe(false);
  });

  it("throws 409 turn_in_progress before the stream with its code and message", async () => {
    const backend = new FakeBackend();
    backend.next["/chat/conversations/c1/messages"] = [json(409, { error: "turn_in_progress", message: "a turn of this conversation is running" })];

    const failure = await client(backend).send("c1", "halo", () => {}).catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(ApiError);
    expect(failure).toMatchObject({ status: 409, code: "turn_in_progress", description: "a turn of this conversation is running" });
  });

  it("throws 503 chat_not_configured", async () => {
    const backend = new FakeBackend();
    backend.chatEnabled = false;

    await expect(client(backend).answer("c1", "call_1", true, () => {})).rejects.toMatchObject({ status: 503, code: "chat_not_configured" });
  });

  it("lists, creates, renames, reads and deletes conversations", async () => {
    const backend = new FakeBackend();
    const chat = client(backend);

    expect((await chat.list()).conversations.map((entry) => entry.id)).toEqual(["c2", "c1"]);
    const created = await chat.create();
    expect(backend.calls("POST", "/chat/conversations")[0].body).toEqual({});
    expect((await chat.rename(created.id, "Baru")).title).toBe("Baru");
    expect((await chat.get(created.id)).conversation.title).toBe("Baru");
    await chat.remove(created.id);
    await expect(chat.get(created.id)).rejects.toMatchObject({ status: 404, code: "not_found" });
  });
});
