import { createHash, randomBytes, timingSafeEqual } from "node:crypto";
import { mkdir, open, readFile, rename, rm } from "node:fs/promises";
import { basename, dirname, join } from "node:path";

export type KeyStatus = "active" | "revoked";

// The plaintext key is handed to NICE once and never stored.
export type KeyRecord = {
  key_id: string;
  label: string;
  sha256: string;
  status: KeyStatus;
  create_time: string;
};

export type IssuedKey = { key_id: string; key: string };

export type KeyStore = {
  generate(label: string): Promise<IssuedKey>;
  rotate(keyId: string): Promise<IssuedKey | undefined>;
  revoke(keyId: string): Promise<boolean>;
  match(digest: Buffer): KeyRecord | undefined;
  activeCount(): number;
};

export class KeyFileError extends Error {}

export function sha256(value: string): Buffer {
  return createHash("sha256").update(value, "utf8").digest();
}

function newKey(): string {
  return "pgk_" + randomBytes(32).toString("base64url");
}

function newKeyId(): string {
  return "key_" + randomBytes(12).toString("hex");
}

export async function openKeyStore(file: string, now: () => Date = () => new Date()): Promise<KeyStore> {
  let records = await load(file);
  let active = index(records);
  let queue: Promise<unknown> = Promise.resolve();

  // Writes run one at a time, and memory changes only after the file is on disk,
  // so a key is never handed out unless it survives a restart.
  function commit<T>(change: (current: KeyRecord[]) => { next: KeyRecord[]; result: T }): Promise<T> {
    const run = queue.then(async () => {
      const { next, result } = change(records);
      if (next !== records) {
        await writeAtomic(file, next);
        records = next;
        active = index(next);
      }
      return result;
    });
    queue = run.catch(() => undefined);
    return run;
  }

  return {
    generate(label) {
      return commit((current) => {
        const key = newKey();
        const record: KeyRecord = {
          key_id: newKeyId(),
          label,
          sha256: sha256(key).toString("hex"),
          status: "active",
          create_time: now().toISOString(),
        };
        return { next: [...current, record], result: { key_id: record.key_id, key } };
      });
    },
    rotate(keyId) {
      return commit((current) => {
        const at = current.findIndex((r) => r.key_id === keyId && r.status === "active");
        if (at < 0) return { next: current, result: undefined };
        const key = newKey();
        const next = current.slice();
        next[at] = { ...current[at]!, sha256: sha256(key).toString("hex") };
        return { next, result: { key_id: keyId, key } };
      });
    },
    revoke(keyId) {
      return commit((current) => {
        const at = current.findIndex((r) => r.key_id === keyId);
        if (at < 0) return { next: current, result: false };
        if (current[at]!.status === "revoked") return { next: current, result: true };
        const next = current.slice();
        next[at] = { ...current[at]!, status: "revoked" };
        return { next, result: true };
      });
    },
    match(digest) {
      // Compare against every active key without stopping early.
      let found: KeyRecord | undefined;
      for (const { record, hash } of active) {
        if (timingSafeEqual(digest, hash) && !found) found = record;
      }
      return found;
    },
    activeCount() {
      return active.length;
    },
  };
}

function index(records: KeyRecord[]): { record: KeyRecord; hash: Buffer }[] {
  return records
    .filter((r) => r.status === "active")
    .map((record) => ({ record, hash: Buffer.from(record.sha256, "hex") }));
}

async function load(file: string): Promise<KeyRecord[]> {
  let text: string;
  try {
    text = await readFile(file, "utf8");
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") return [];
    throw err;
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new KeyFileError(`${file} is not valid JSON`);
  }
  const keys = (parsed as { keys?: unknown } | null)?.keys;
  if (!Array.isArray(keys) || !keys.every(isRecord)) {
    throw new KeyFileError(`${file} does not hold a valid key list`);
  }
  return keys;
}

function isRecord(v: unknown): v is KeyRecord {
  if (typeof v !== "object" || v === null) return false;
  const r = v as Record<string, unknown>;
  return (
    typeof r.key_id === "string" &&
    typeof r.label === "string" &&
    typeof r.sha256 === "string" &&
    /^[0-9a-f]{64}$/.test(r.sha256) &&
    (r.status === "active" || r.status === "revoked") &&
    typeof r.create_time === "string"
  );
}

// Temp file in the same directory, fsync, then rename: readers see the old file or the new one, never half of one.
async function writeAtomic(file: string, records: KeyRecord[]): Promise<void> {
  const dir = dirname(file);
  await mkdir(dir, { recursive: true });
  const tmp = join(dir, `.${basename(file)}.${process.pid}.${randomBytes(4).toString("hex")}.tmp`);
  try {
    const fh = await open(tmp, "w", 0o600);
    try {
      await fh.writeFile(JSON.stringify({ keys: records }, null, 2) + "\n");
      await fh.sync();
    } finally {
      await fh.close();
    }
    await rename(tmp, file);
  } catch (err) {
    await rm(tmp, { force: true });
    throw err;
  }
}
