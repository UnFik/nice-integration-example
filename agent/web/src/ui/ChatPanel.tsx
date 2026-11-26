import { type FormEvent, useEffect, useRef, useState } from "react";
import { type Message, type ToolCall, type ToolResult, titleOf } from "../chat";
import type { Session, SessionState } from "../session";
import { pretty } from "./format";
import { Icon } from "./icons";

export const MAX_MESSAGE = 8000;

// Every text here (the user's, the model's, tool results, titles) is rendered
// as a React text node: the model and the tools are not trusted to send HTML.
export function ChatPanel({ session, state }: { session: Session; state: SessionState }) {
  const thread = state.selectedId ? state.thread : undefined;
  const running = state.turn !== undefined && state.turn.conversationId === state.selectedId;
  const results = resultsOf(thread?.messages ?? []);
  const end = useRef<HTMLDivElement>(null);
  const count = thread?.messages.length ?? 0;

  useEffect(() => {
    end.current?.scrollIntoView?.({ block: "end" });
  }, [count, state.selectedId]);

  return (
    <main id="percakapan" className="chat-panel" aria-labelledby="chat-title">
      <header className="chat-head">
        <h1 id="chat-title">{thread ? titleOf(thread.conversation) : state.selectedId ? "Memuat…" : "Percakapan baru"}</h1>
        {thread && (
          <button type="button" className="quiet" disabled={state.threadLoading || running} onClick={() => void session.reloadThread()}>
            <Icon name="refresh" />
            Muat ulang
          </button>
        )}
      </header>

      <div className="messages" aria-live="polite" aria-busy={running || undefined}>
        {state.threadError ? (
          <div className="callout error" role="alert">
            <Icon name="alert" />
            <span>
              {state.threadError}{" "}
              <button type="button" className="quiet" onClick={() => void session.reloadThread()}>
                Coba lagi
              </button>
            </span>
          </div>
        ) : state.selectedId && !thread ? (
          <p className="inline-status loading">
            <Icon name="spinner" className="spin" />
            Memuat percakapan…
          </p>
        ) : !thread || thread.messages.length === 0 ? (
          <div className="empty">
            <Icon name="chat" />
            <p>Minta agen, mis. "tampilkan daftar perangkat di nadi". Operasi sensitif menunggu persetujuan Anda di sini.</p>
          </div>
        ) : (
          thread.messages.map((message) => (
            <MessageView key={message.id} message={message} results={results} state={state} running={running} session={session} />
          ))
        )}
        {running && <TurnStatus state={state} results={results} />}
        <div ref={end} />
      </div>

      {state.chatError && (
        <p className="callout error chat-error" role="alert">
          <Icon name="alert" />
          <span>{state.chatError}</span>
        </p>
      )}

      <Composer session={session} state={state} running={running} />
    </main>
  );
}

function resultsOf(messages: Message[]): Map<string, ToolResult> {
  const results = new Map<string, ToolResult>();
  for (const message of messages) {
    for (const part of message.parts) if (part.kind === "tool_result") results.set(part.result.callId, part.result);
  }
  return results;
}

function MessageView(props: { message: Message; results: Map<string, ToolResult>; state: SessionState; running: boolean; session: Session }) {
  const { message } = props;
  if (message.role === "tool") return null;
  if (message.role === "user") {
    return (
      <article className="message user" data-role="user">
        <span className="visually-hidden">Anda: </span>
        {message.parts.map((part, index) => part.kind === "text" && <p key={index}>{part.text}</p>)}
      </article>
    );
  }
  return (
    <article className="message assistant" data-role="assistant">
      <span className="visually-hidden">Agen: </span>
      {message.parts.map((part, index) => {
        if (part.kind === "text") return part.text ? <p key={index}>{part.text}</p> : null;
        if (part.kind === "tool_call") {
          const result = props.results.get(part.call.id);
          const approval = props.state.approvals[part.call.id] ?? (part.call.status === "awaiting_approval" ? part.call.approvalMessage : undefined);
          if (!result && approval !== undefined) {
            return <ApprovalCard key={index} call={part.call} message={approval} state={props.state} session={props.session} />;
          }
          return <ToolCard key={index} call={part.call} result={result} running={props.running} />;
        }
        return null;
      })}
    </article>
  );
}

// summary names what a tool call is about: the service and the operation, not its whole input.
export function summary(call: ToolCall): string {
  const service = typeof call.input.service === "string" ? call.input.service : "";
  const operation = typeof call.input.operation === "string" ? call.input.operation : "";
  return [service, operation].filter(Boolean).join(" · ");
}

function ToolCard({ call, result, running }: { call: ToolCall; result?: ToolResult; running: boolean }) {
  const status = result ? (result.isError ? "error" : "done") : call.status === "done" ? "done" : running ? "running" : "none";
  const label = { error: "Gagal", done: "Selesai", running: "Berjalan…", none: "Tanpa hasil" }[status];
  const about = summary(call);
  return (
    <section className={`tool-card ${status}`} data-call={call.id} data-status={status}>
      <header className="tool-head">
        <Icon name="tool" />
        <code>{call.name}</code>
        {about && <span className="muted small tool-about">{about}</span>}
        <span className={`tool-status ${status}`}>
          <Icon name={status === "running" ? "spinner" : status === "error" ? "alert" : status === "done" ? "check" : "info"} className={status === "running" ? "spin" : undefined} />
          {label}
        </span>
      </header>
      <details className="plain-details tool-details">
        <summary>Rincian</summary>
        <p className="muted small">Input</p>
        <pre>{pretty(call.input)}</pre>
        {result && (
          <>
            <p className="muted small">{result.isError ? "Galat" : "Hasil"}</p>
            <pre>{pretty(result.content)}</pre>
          </>
        )}
      </details>
    </section>
  );
}

// ApprovalCard shows the message the backend built from the catalog, never
// text of the model, so the user decides on what will really be called.
function ApprovalCard({ call, message, state, session }: { call: ToolCall; message: string; state: SessionState; session: Session }) {
  const busy = state.answering !== undefined || state.turn !== undefined;
  const about = summary(call);
  return (
    <section className="approval-card" data-approval={call.id} aria-labelledby={`approval-${call.id}`}>
      <header className="tool-head">
        <Icon name="shield" />
        <strong id={`approval-${call.id}`}>Butuh persetujuan Anda</strong>
        {about && <span className="muted small tool-about">{about}</span>}
      </header>
      <p className="approval-message">{message || "Operasi ini sensitif dan menunggu persetujuan."}</p>
      <details className="plain-details tool-details">
        <summary>Input panggilan</summary>
        <pre>{pretty(call.input)}</pre>
      </details>
      <div className="actions">
        <button type="button" className="primary" data-answer="approve" disabled={busy} onClick={() => void session.answer(call.id, true)}>
          {state.answering === call.id ? <Icon name="spinner" className="spin" /> : <Icon name="check" />}
          Setujui
        </button>
        <button type="button" data-answer="decline" disabled={busy} onClick={() => void session.answer(call.id, false)}>
          <Icon name="x" />
          Tolak
        </button>
      </div>
    </section>
  );
}

function TurnStatus({ state, results }: { state: SessionState; results: Map<string, ToolResult> }) {
  const tools = (state.turn?.tools ?? []).filter((tool) => !results.has(tool.callId));
  return (
    <p className="inline-status turn-status" role="status">
      <Icon name="spinner" className="spin" />
      {tools.length > 0 ? `Menjalankan ${tools.map((tool) => tool.name).join(", ")}…` : "Agen sedang bekerja…"}
    </p>
  );
}

function Composer({ session, state, running }: { session: Session; state: SessionState; running: boolean }) {
  const [draft, setDraft] = useState("");
  const usable = state.active !== undefined && !state.contextError;
  const off = state.chatEnabled === false;
  const disabled = !usable || off || state.turn !== undefined || state.answering !== undefined;
  const tooLong = [...draft].length > MAX_MESSAGE;

  const submit = async (event?: FormEvent) => {
    event?.preventDefault();
    if (disabled || tooLong || !draft.trim()) return;
    const text = draft;
    setDraft("");
    const ok = await session.send(text);
    if (!ok) setDraft((current) => current || text);
  };

  return (
    <form className="composer" onSubmit={(event) => void submit(event)}>
      {off && (
        <p className="callout warning" role="status">
          <Icon name="info" />
          <span>Chat belum dikonfigurasi di agen. Riwayat tetap bisa dibaca, tetapi pesan baru tidak bisa dikirim.</span>
        </p>
      )}
      <label htmlFor="pesan" className="visually-hidden">
        Pesan untuk agen
      </label>
      <div className="composer-row">
        <textarea
          id="pesan"
          rows={2}
          value={draft}
          disabled={disabled}
          aria-invalid={tooLong || undefined}
          placeholder={running ? "Menunggu giliran selesai…" : usable ? "Tulis pesan. Enter untuk kirim, Shift+Enter untuk baris baru." : "Pilih project lebih dulu."}
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
              event.preventDefault();
              void submit();
            }
          }}
        />
        <button type="submit" className="primary" disabled={disabled || tooLong || !draft.trim()}>
          <Icon name="send" />
          Kirim
        </button>
      </div>
      {tooLong && <p className="field-error">Pesan lebih dari {MAX_MESSAGE} karakter.</p>}
    </form>
  );
}
