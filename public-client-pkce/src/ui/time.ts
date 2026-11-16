export function clock(ms: number): string {
  return new Date(ms).toLocaleTimeString("id-ID");
}

export function remaining(ms: number): string {
  const total = Math.floor(ms / 1000);
  if (total <= 0) return "0 dtk";
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  if (hours > 0) return `${hours} j ${String(minutes).padStart(2, "0")} mnt`;
  if (minutes > 0) return `${minutes} mnt ${String(seconds).padStart(2, "0")} dtk`;
  return `${seconds} dtk`;
}
