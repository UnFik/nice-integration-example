import { useCallback, useEffect, useRef, useState } from "react";
import { Icon } from "./icons";

export interface Toast {
  id: number;
  kind: "success" | "error";
  text: string;
}

const SUCCESS_MS = 6000;

export function useToasts() {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const next = useRef(1);
  const dismiss = useCallback((id: number) => setToasts((all) => all.filter((toast) => toast.id !== id)), []);
  const notify = useCallback(
    (kind: Toast["kind"], text: string) => {
      const id = next.current++;
      setToasts((all) => [...all.slice(-2), { id, kind, text }]);
      if (kind === "success") window.setTimeout(() => dismiss(id), SUCCESS_MS);
    },
    [dismiss],
  );
  return { toasts, notify, dismiss };
}

// The region stays in the page while empty, so a screen reader announces what arrives in it.
export function ToastRegion({ toasts, onDismiss }: { toasts: Toast[]; onDismiss: (id: number) => void }) {
  return (
    <div className="toasts" role="status" aria-live="polite" aria-atomic="false">
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} onDismiss={onDismiss} />
      ))}
    </div>
  );
}

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: (id: number) => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => ref.current?.setAttribute("data-shown", ""), []);
  return (
    <div ref={ref} className={`toast ${toast.kind}`}>
      <Icon name={toast.kind === "success" ? "check" : "alert"} />
      <p>
        <span className="visually-hidden">{toast.kind === "success" ? "Berhasil: " : "Gagal: "}</span>
        {toast.text}
      </p>
      <button type="button" className="icon-button" aria-label="Tutup pemberitahuan" onClick={() => onDismiss(toast.id)}>
        <Icon name="x" />
      </button>
    </div>
  );
}
