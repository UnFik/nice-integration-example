import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach } from "vitest";
import { buildApp } from "../src/app.ts";
import { formatLine, type Logger } from "../src/log.ts";

export const SERVICE_CREDENTIAL = "test-service-credential-1f9c2b7d";

const dirs: string[] = [];

afterEach(async () => {
  await Promise.all(dirs.splice(0).map((d) => rm(d, { recursive: true, force: true })));
});

export function memoryLogger(): Logger & { lines: string[] } {
  const lines: string[] = [];
  return {
    lines,
    info: (msg, fields) => void lines.push(formatLine("INFO", msg, fields)),
    warn: (msg, fields) => void lines.push(formatLine("WARN", msg, fields)),
    error: (msg, fields) => void lines.push(formatLine("ERROR", msg, fields)),
  };
}

export async function tempDir(): Promise<string> {
  const dir = await mkdtemp(join(tmpdir(), "pengiriman-test-"));
  dirs.push(dir);
  return dir;
}

// Opens the app on a key file in a fresh temp dir; restart() builds a new app on the same file.
export async function setup() {
  const dir = await tempDir();
  const keysFile = join(dir, "data", "keys.json");
  const log = memoryLogger();
  const start = () => buildApp({ serviceCredential: SERVICE_CREDENTIAL, keysFile, log });
  let current = await start();
  return {
    dir,
    keysFile,
    log,
    get app() {
      return current.app;
    },
    get keys() {
      return current.keys;
    },
    async restart() {
      current = await start();
    },
  };
}

export function headers(key: string | null, org: string | null = "org_A", principal: string | null = "sa_X") {
  const h: Record<string, string> = { "Content-Type": "application/json" };
  if (key !== null) h["X-Api-Key"] = key;
  if (org !== null) h["X-Nice-Organization"] = org;
  if (principal !== null) h["X-Nice-Principal"] = principal;
  return h;
}

export async function generateKey(app: { request: (path: string, init?: RequestInit) => Response | Promise<Response> }, label = "sa_X") {
  const res = await app.request("/keys", {
    method: "POST",
    headers: headers(SERVICE_CREDENTIAL, null, null),
    body: JSON.stringify({ label }),
  });
  if (res.status !== 201) throw new Error(`generate answered ${res.status}`);
  return (await res.json()) as { key_id: string; key: string };
}
