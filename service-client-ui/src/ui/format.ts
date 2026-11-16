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

export function absoluteTime(iso: string | undefined): string {
  const at = iso ? Date.parse(iso) : Number.NaN;
  return Number.isNaN(at) ? "-" : new Date(at).toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "medium" });
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

// countdown writes milliseconds as m:ss, or h:mm:ss from an hour on.
export function countdown(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  const pad = (value: number) => String(value).padStart(2, "0");
  return hours > 0 ? `${hours}:${pad(minutes)}:${pad(seconds)}` : `${minutes}:${pad(seconds)}`;
}
