const relative = new Intl.RelativeTimeFormat("id-ID", { numeric: "auto" });

// relativeTime says how long ago an ISO timestamp was, in the largest unit that fits.
export function relativeTime(iso: string | undefined, now: number): string {
  const at = iso ? Date.parse(iso) : Number.NaN;
  if (Number.isNaN(at)) return "-";
  const seconds = Math.round((at - now) / 1000);
  const abs = Math.abs(seconds);
  if (abs < 45) return "baru saja";
  if (abs < 3600) return relative.format(Math.round(seconds / 60), "minute");
  if (abs < 86_400) return relative.format(Math.round(seconds / 3600), "hour");
  if (abs < 30 * 86_400) return relative.format(Math.round(seconds / 86_400), "day");
  return new Date(at).toLocaleDateString("id-ID", { day: "numeric", month: "short", year: "numeric" });
}

// shortId keeps the prefix and the tail of an id, which is enough to tell two apart.
export function shortId(id: string | undefined): string {
  if (!id) return "-";
  return id.length > 15 ? `${id.slice(0, 8)}…${id.slice(-4)}` : id;
}

export function initials(name: string | undefined): string {
  const words = (name ?? "").trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return "?";
  return words
    .slice(0, 2)
    .map((word) => [...word][0].toUpperCase())
    .join("");
}

// pretty prints a tool input or result for the collapsible view; it stays text.
export function pretty(value: unknown): string {
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2) ?? "null";
  } catch {
    return String(value);
  }
}
