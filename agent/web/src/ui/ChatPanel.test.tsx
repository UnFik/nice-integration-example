import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { Message, Thread } from "../chat";
import { Session, type SessionState } from "../session";
import { ORG_A, PRJ_P } from "../test/fake-backend";
import { ChatPanel } from "./ChatPanel";

const session = new Session({ fetch: () => Promise.reject(new Error("offline")), navigate: () => {}, replaceUrl: () => {} });

const HOSTILE = "<img src=x onerror=alert(1)>";

const conversation = { id: "c1", title: "Gudang", createdAt: "2026-11-01T10:00:00Z", updatedAt: "2026-11-02T10:00:00Z" };

function message(id: string, role: Message["role"], parts: Message["parts"]): Message {
  return { id, seq: 1, role, createdAt: "2026-11-02T10:00:00Z", parts };
}

const call = { id: "call_1", name: "call_operation", input: { service: "nadi", operation: "deleteDevice", path: { id: "dev_1" } }, status: "" as const };

function state(messages: Message[], patch: Partial<SessionState> = {}): SessionState {
  const thread: Thread = { conversation, messages };
  return {
    phase: "signed-in",
    perms: ["agen.mcp.use"],
    active: { org: ORG_A, project: PRJ_P },
    approvals: {},
    conversations: [conversation],
    chatEnabled: true,
    selectedId: "c1",
    thread,
    ...patch,
  };
}

const render = (value: SessionState) => renderToStaticMarkup(<ChatPanel session={session} state={value} />);

describe("ChatPanel", () => {
  it("shows HTML from the user, the model, a tool and a title as text, with no img element", () => {
    const markup = render({
      ...state([]),
      thread: {
        conversation: { ...conversation, title: HOSTILE },
        messages: [
          message("m1", "user", [{ kind: "text", text: HOSTILE }]),
          message("m2", "assistant", [{ kind: "text", text: HOSTILE }, { kind: "tool_call", call: { ...call, input: { service: HOSTILE } } }]),
          message("m3", "tool", [{ kind: "tool_result", result: { callId: "call_1", content: { body: HOSTILE }, isError: false } }]),
        ],
      },
    });

    expect(markup).not.toMatch(/<img/i);
    expect(markup).toContain("&lt;img src=x onerror=alert(1)&gt;");
    expect(markup.match(/&lt;img src=x onerror=alert\(1\)&gt;/g)?.length).toBeGreaterThanOrEqual(4);
  });

  it("shows a tool call as a card with name, service and operation, and its result", () => {
    const markup = render(
      state([
        message("m2", "assistant", [{ kind: "tool_call", call }]),
        message("m3", "tool", [{ kind: "tool_result", result: { callId: "call_1", content: { status: 204 }, isError: false } }]),
      ]),
    );

    expect(markup).toMatch(/data-call="call_1" data-status="done"/);
    expect(markup).toContain("<code>call_operation</code>");
    expect(markup).toContain("nadi · deleteDevice");
    expect(markup).toContain("&quot;status&quot;: 204");
  });

  it("marks a failed tool", () => {
    const markup = render(
      state([
        message("m2", "assistant", [{ kind: "tool_call", call }]),
        message("m3", "tool", [{ kind: "tool_result", result: { callId: "call_1", content: { error: "service_not_allowed" }, isError: true } }]),
      ]),
    );

    expect(markup).toMatch(/data-status="error"/);
    expect(markup).toContain("Gagal");
  });

  it("shows the catalog message of a pending call with Setujui and Tolak", () => {
    const markup = render(
      state([message("m2", "assistant", [{ kind: "tool_call", call: { ...call, status: "awaiting_approval", approvalMessage: "Hapus perangkat dev_1 di nadi?" } }])]),
    );

    expect(markup).toContain('data-approval="call_1"');
    expect(markup).toContain("Hapus perangkat dev_1 di nadi?");
    expect(markup).toMatch(/data-answer="approve"[^>]*>.*Setujui/);
    expect(markup).toMatch(/data-answer="decline"[^>]*>.*Tolak/);
    expect(markup).not.toMatch(/<button[^>]*data-answer="approve"[^>]*disabled/);
  });

  it("shows an approval from the stream before the history is reloaded", () => {
    const markup = render(state([message("m2", "assistant", [{ kind: "tool_call", call }])], { approvals: { call_1: "Hapus?" } }));

    expect(markup).toContain('data-approval="call_1"');
  });

  it("disables the answer buttons and the input while an answer is sent", () => {
    const markup = render(
      state([message("m2", "assistant", [{ kind: "tool_call", call: { ...call, status: "awaiting_approval", approvalMessage: "Hapus?" } }])], {
        answering: "call_1",
        turn: { conversationId: "c1", tools: [] },
      }),
    );

    expect(markup).toMatch(/<button[^>]*data-answer="approve"[^>]*disabled=""/);
    expect(markup).toMatch(/<button[^>]*data-answer="decline"[^>]*disabled=""/);
    expect(markup).toMatch(/<textarea[^>]*disabled=""/);
    expect(markup).toMatch(/Agen sedang bekerja/);
  });

  it("keeps the input on when idle", () => {
    expect(render(state([]))).not.toMatch(/<textarea[^>]*disabled=""/);
  });

  it("says the chat is not configured and disables the input", () => {
    const markup = render(state([], { chatEnabled: false }));

    expect(markup).toContain("Chat belum dikonfigurasi di agen");
    expect(markup).toMatch(/<textarea[^>]*disabled=""/);
  });

  it("shows an error of the turn", () => {
    expect(render(state([], { chatError: "Giliran sebelumnya di percakapan ini masih berjalan." }))).toMatch(/role="alert".*masih berjalan/);
  });
});
