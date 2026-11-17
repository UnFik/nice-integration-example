import { useEffect, useState } from "react";
import { Icon } from "./icons";

export function CopyButton({ value, label }: { value: string; label: string }) {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");

  useEffect(() => {
    if (state === "idle") return;
    const id = window.setTimeout(() => setState("idle"), 1800);
    return () => window.clearTimeout(id);
  }, [state]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setState("copied");
    } catch {
      setState("failed");
    }
  }

  const text = state === "copied" ? "Tersalin" : state === "failed" ? "Gagal menyalin" : label;
  return (
    <button type="button" className="btn btn-ghost btn-sm" onClick={() => void copy()}>
      <Icon name={state === "copied" ? "check" : "copy"} size={16} />
      <span role="status">{text}</span>
    </button>
  );
}
