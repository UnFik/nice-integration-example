export type LogFields = Record<string, string | number | boolean | null | undefined>;

export type Logger = {
  info(msg: string, fields?: LogFields): void;
  warn(msg: string, fields?: LogFields): void;
  error(msg: string, fields?: LogFields): void;
};

export function formatLine(level: string, msg: string, fields: LogFields = {}): string {
  const parts = [new Date().toISOString(), level, msg];
  for (const [k, v] of Object.entries(fields)) {
    if (v === undefined) continue;
    const s = String(v);
    parts.push(`${k}=${/^[\w.:/@-]+$/.test(s) ? s : JSON.stringify(s)}`);
  }
  return parts.join(" ");
}

// Callers pass only ids, paths and status codes; never a header value or a key.
export function consoleLogger(): Logger {
  return {
    info: (msg, fields) => process.stdout.write(formatLine("INFO", msg, fields) + "\n"),
    warn: (msg, fields) => process.stderr.write(formatLine("WARN", msg, fields) + "\n"),
    error: (msg, fields) => process.stderr.write(formatLine("ERROR", msg, fields) + "\n"),
  };
}
