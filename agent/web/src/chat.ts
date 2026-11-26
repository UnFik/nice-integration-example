import { type Api, ApiError, record, text } from "./api";

export interface Conversation {
  id: string;
  title: string;
  createdAt: string;
  updatedAt: string;
}

export type CallStatus = "" | "done" | "awaiting_approval";

export interface ToolCall {
  id: string;
  name: string;
  input: Record<string, unknown>;
  status: CallStatus;
  approvalMessage?: string;
}

export interface ToolResult {
  callId: string;
  content: unknown;
  isError: boolean;
}

export type Part =
  | { kind: "text"; text: string }
  | { kind: "tool_call"; call: ToolCall }
  | { kind: "tool_result"; result: ToolResult };

export type Role = "user" | "assistant" | "tool";

export interface Message {
  id: string;
  seq: number;
  role: Role;
  createdAt: string;
  parts: Part[];
}

export interface ConversationList {
  conversations: Conversation[];
  chatEnabled: boolean;
}

export interface Thread {
  conversation: Conversation;
  messages: Message[];
}

export type DoneReason = "answered" | "awaiting_approval" | "step_limit";

export type ChatEvent =
  | { type: "assistant"; message: Message }
  | { type: "tool_call"; callId: string; name: string }
  | { type: "tool_result"; message: Message }
  | { type: "approval"; callId: string; message: string }
  | { type: "done"; reason: DoneReason | string }
  | { type: "error"; code: string; message: string };

export type ChatApi = ReturnType<typeof createChatApi>;

// createChatApi wraps /chat/*. A turn (send, answer) streams its steps; the
// promise resolves when the stream ends, with whether a done or error event came.
export function createChatApi(api: Api) {
  const conversationPath = (id: string) => `/chat/conversations/${encodeURIComponent(id)}`;
  const stream = async (path: string, body: unknown, onEvent: (event: ChatEvent) => void): Promise<boolean> => {
    const response = await api.open(path, body);
    let ended = false;
    await readEvents(response.body as ReadableStream<Uint8Array>, (event) => {
      if (event.type === "done" || event.type === "error") ended = true;
      onEvent(event);
    });
    return ended;
  };

  return {
    async list(): Promise<ConversationList> {
      const value = record(await api.send("/chat/conversations", "GET"));
      if (!Array.isArray(value.conversations)) throw new ApiError(200, "invalid_body", "conversations");
      return { conversations: value.conversations.map(toConversation), chatEnabled: value.chat_enabled === true };
    },
    async create(title?: string): Promise<Conversation> {
      const value = record(await api.send("/chat/conversations", "POST", title ? { title } : {}));
      return toConversation(value.conversation);
    },
    async get(id: string): Promise<Thread> {
      const value = record(await api.send(conversationPath(id), "GET"));
      return {
        conversation: toConversation(value.conversation),
        messages: Array.isArray(value.messages) ? value.messages.map(toMessage) : [],
      };
    },
    async rename(id: string, title: string): Promise<Conversation> {
      return toConversation(record(await api.send(conversationPath(id), "PATCH", { title })).conversation);
    },
    async remove(id: string): Promise<void> {
      await api.send(conversationPath(id), "DELETE");
    },
    send(id: string, message: string, onEvent: (event: ChatEvent) => void): Promise<boolean> {
      return stream(`${conversationPath(id)}/messages`, { text: message }, onEvent);
    },
    answer(id: string, callId: string, approve: boolean, onEvent: (event: ChatEvent) => void): Promise<boolean> {
      return stream(`${conversationPath(id)}/approvals/${encodeURIComponent(callId)}`, { approve }, onEvent);
    },
  };
}

// readEvents reads server-sent events from a fetch body. EventSource cannot
// POST, so the frames are cut here: a frame may span chunks, and one chunk may
// hold several frames.
export async function readEvents(body: ReadableStream<Uint8Array>, onEvent: (event: ChatEvent) => void): Promise<void> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  const drain = () => {
    buffer = buffer.replace(/\r\n?/g, "\n");
    let end = buffer.indexOf("\n\n");
    while (end >= 0) {
      const event = parseFrame(buffer.slice(0, end));
      buffer = buffer.slice(end + 2);
      if (event) onEvent(event);
      end = buffer.indexOf("\n\n");
    }
  };
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    drain();
  }
  buffer += decoder.decode();
  // The last frame may miss its blank line when the server closed right after it.
  if (buffer.trim()) buffer += "\n\n";
  drain();
}

export function parseFrame(frame: string): ChatEvent | undefined {
  let type = "message";
  const data: string[] = [];
  for (const line of frame.split("\n")) {
    if (!line || line.startsWith(":")) continue;
    const colon = line.indexOf(":");
    const field = colon < 0 ? line : line.slice(0, colon);
    const value = colon < 0 ? "" : line.slice(colon + 1).replace(/^ /, "");
    if (field === "event") type = value;
    if (field === "data") data.push(value);
  }
  let payload: Record<string, unknown>;
  try {
    payload = record(JSON.parse(data.join("\n")));
  } catch {
    return undefined;
  }
  switch (type) {
    case "assistant":
    case "tool_result":
      return { type, message: toMessage(payload.message) };
    case "tool_call":
      return { type, callId: text(payload.call_id), name: text(payload.name) };
    case "approval":
      return { type, callId: text(payload.call_id), message: text(payload.message) };
    case "done":
      return { type, reason: text(payload.reason) };
    case "error":
      return { type, code: text(payload.error) || "internal", message: text(payload.message) };
    default:
      return undefined;
  }
}

export function toConversation(raw: unknown): Conversation {
  const value = record(raw);
  return { id: text(value.id), title: text(value.title), createdAt: text(value.created_at), updatedAt: text(value.updated_at) };
}

export function toMessage(raw: unknown): Message {
  const value = record(raw);
  const role = value.role === "user" || value.role === "tool" ? value.role : "assistant";
  const parts = Array.isArray(value.parts) ? value.parts.map(toPart).filter((part): part is Part => part !== undefined) : [];
  return { id: text(value.id), seq: typeof value.seq === "number" ? value.seq : 0, role, createdAt: text(value.created_at), parts };
}

function toPart(raw: unknown): Part | undefined {
  const value = record(raw);
  if (value.kind === "text") return { kind: "text", text: text(value.text) };
  if (value.kind === "tool_call") {
    const call = record(value.call);
    const status = call.status === "done" || call.status === "awaiting_approval" ? call.status : "";
    return {
      kind: "tool_call",
      call: {
        id: text(call.id),
        name: text(call.name),
        input: record(call.input),
        status,
        approvalMessage: text(call.approval_message) || undefined,
      },
    };
  }
  if (value.kind === "tool_result") {
    const result = record(value.result);
    return { kind: "tool_result", result: { callId: text(result.call_id), content: result.content ?? null, isError: result.is_error === true } };
  }
  return undefined;
}

export const TITLE_FALLBACK = "Percakapan baru";

export function titleOf(conversation: Conversation): string {
  return conversation.title.trim() || TITLE_FALLBACK;
}

// describeChatError turns a code of /chat/* into a sentence for the user.
export function describeChatError(code: string | undefined, message?: string): string {
  switch (code) {
    case "turn_in_progress":
      return "Giliran sebelumnya di percakapan ini masih berjalan. Tunggu sampai selesai, lalu muat ulang percakapan.";
    case "call_not_pending":
      return "Panggilan ini tidak lagi menunggu persetujuan, mis. karena sudah dijawab. Riwayat dimuat ulang.";
    case "chat_not_configured":
      return "Chat belum dikonfigurasi di agen.";
    case "message_too_long":
      return "Pesan terlalu panjang (maksimal 8000 karakter).";
    case "invalid_input":
      return `Isian ditolak backend${message ? ` (${message})` : ""}.`;
    case "not_found":
      return "Percakapan tidak ditemukan. Mungkin sudah dihapus.";
    case "model_unavailable":
      return "Model LLM sedang tidak tersedia. Coba lagi sebentar lagi.";
    case "model_refused":
      return "Provider LLM menolak permintaan. Periksa pengaturan LLM di agen.";
    case "timeout":
      return "Giliran terlalu lama dan dihentikan.";
    case "insufficient_scope":
    case "permission_denied":
      return "Token konteks aktif tidak memuat izin agen.mcp.use, jadi chat tidak dapat dipakai di konteks ini.";
    default:
      return `Permintaan chat gagal${code ? ` (${code})` : ""}${message ? `: ${message}` : ""}.`;
  }
}
