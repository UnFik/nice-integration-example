import { readdir, readFile, writeFile, mkdir } from "node:fs/promises";
import { dirname } from "node:path";
import { describe, expect, it } from "vitest";
import { buildApp } from "../src/app.ts";
import { KeyFileError } from "../src/keys.ts";
import { generateKey, headers, memoryLogger, SERVICE_CREDENTIAL, setup } from "./helpers.ts";

const admin = () => headers(SERVICE_CREDENTIAL, null, null);

const listAs = (app: { request: (p: string, i?: RequestInit) => Response | Promise<Response> }, key: string) =>
  app.request("/shipments", { headers: headers(key) });

describe("key operations", () => {
  it("generates a key that is accepted at once", async () => {
    const { app } = await setup();
    const res = await app.request("/keys", { method: "POST", headers: admin(), body: JSON.stringify({ label: "sa_X" }) });
    expect(res.status).toBe(201);
    const body = (await res.json()) as { key_id: string; key: string };
    expect(Object.keys(body).sort()).toEqual(["key", "key_id"]);
    expect(body.key_id).toMatch(/^key_[0-9a-f]{24}$/);
    expect(body.key).toMatch(/^pgk_[A-Za-z0-9_-]{43}$/);
    expect((await listAs(app, body.key)).status).toBe(200);
  });

  it("does not need X-Nice-* headers, since NICE sends none to the key operations", async () => {
    const { app } = await setup();
    const res = await app.request("/keys", {
      method: "POST",
      headers: { "X-Api-Key": SERVICE_CREDENTIAL, "Content-Type": "application/json" },
      body: JSON.stringify({ label: "sa_X" }),
    });
    expect(res.status).toBe(201);
  });

  it("rejects a missing or blank label and a body that is not JSON", async () => {
    const { app } = await setup();
    for (const body of ["{}", JSON.stringify({ label: "  " }), JSON.stringify({ label: 7 }), JSON.stringify({ label: "x".repeat(201) })]) {
      const res = await app.request("/keys", { method: "POST", headers: admin(), body });
      expect(res.status, body).toBe(400);
      expect(await res.json()).toEqual({ error: "invalid_argument", error_description: "label" });
    }
    const res = await app.request("/keys", { method: "POST", headers: admin(), body: "{" });
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({ error: "invalid_argument", error_description: "body" });
  });

  it("rotates: same key_id, new key, the old key dies at once", async () => {
    const { app } = await setup();
    const first = await generateKey(app);
    // NICE sends {"key_id"} in the rotate body; the service accepts it.
    const res = await app.request(`/keys/${first.key_id}/rotate`, {
      method: "POST",
      headers: admin(),
      body: JSON.stringify({ key_id: first.key_id }),
    });
    expect(res.status).toBe(200);
    const rotated = (await res.json()) as { key_id: string; key: string };
    expect(rotated.key_id).toBe(first.key_id);
    expect(rotated.key).not.toBe(first.key);
    expect((await listAs(app, first.key)).status).toBe(401);
    expect((await listAs(app, rotated.key)).status).toBe(200);
  });

  it("revokes with 204 and the key stops working", async () => {
    const { app } = await setup();
    const k = await generateKey(app);
    const res = await app.request(`/keys/${k.key_id}`, { method: "DELETE", headers: { "X-Api-Key": SERVICE_CREDENTIAL } });
    expect(res.status).toBe(204);
    expect(await res.text()).toBe("");
    expect((await listAs(app, k.key)).status).toBe(401);
  });

  it("answers 404 for an unknown key id, and for rotating a revoked key", async () => {
    const { app } = await setup();
    const del = await app.request("/keys/key_unknown", { method: "DELETE", headers: admin() });
    expect(del.status).toBe(404);
    expect(await del.json()).toEqual({ error: "not_found" });
    expect((await app.request("/keys/key_unknown/rotate", { method: "POST", headers: admin() })).status).toBe(404);

    const k = await generateKey(app);
    await app.request(`/keys/${k.key_id}`, { method: "DELETE", headers: admin() });
    expect((await app.request(`/keys/${k.key_id}/rotate`, { method: "POST", headers: admin() })).status).toBe(404);
  });

  it("refuses a mirrored key on every key operation with 403 and creates nothing", async () => {
    const { app, keysFile } = await setup();
    const k = await generateKey(app);
    const before = await readFile(keysFile, "utf8");
    const asMirrored = { "X-Api-Key": k.key, "Content-Type": "application/json" };
    const calls: [string, string][] = [
      ["POST", "/keys"],
      ["POST", `/keys/${k.key_id}/rotate`],
      ["DELETE", `/keys/${k.key_id}`],
    ];
    for (const [method, path] of calls) {
      const res = await app.request(path, { method, headers: asMirrored, body: method === "POST" ? JSON.stringify({ label: "x" }) : undefined });
      expect(res.status, `${method} ${path}`).toBe(403);
      expect(await res.json()).toEqual({ error: "admin_credential_required" });
    }
    expect(await readFile(keysFile, "utf8")).toBe(before);
    expect((await listAs(app, k.key)).status).toBe(200);
  });

  it("keeps active keys and revoked keys across a restart", async () => {
    const env = await setup();
    const kept = await generateKey(env.app, "sa_keep");
    const gone = await generateKey(env.app, "sa_gone");
    await env.app.request(`/keys/${gone.key_id}`, { method: "DELETE", headers: admin() });

    await env.restart();

    expect((await listAs(env.app, kept.key)).status).toBe(200);
    expect((await listAs(env.app, gone.key)).status).toBe(401);
    expect(env.keys.activeCount()).toBe(1);
  });

  it("stores only hashes, through a temp file and rename", async () => {
    const { app, keysFile } = await setup();
    const a = await generateKey(app, "sa_A");
    const b = await generateKey(app, "sa_B");
    const rotated = (await (await app.request(`/keys/${a.key_id}/rotate`, { method: "POST", headers: admin() })).json()) as {
      key: string;
    };
    const text = await readFile(keysFile, "utf8");
    for (const secret of [a.key, b.key, rotated.key, SERVICE_CREDENTIAL]) {
      expect(text).not.toContain(secret);
      expect(text).not.toContain(secret.slice(4));
    }
    const { keys } = JSON.parse(text) as { keys: Record<string, unknown>[] };
    expect(keys).toHaveLength(2);
    for (const k of keys) {
      expect(Object.keys(k).sort()).toEqual(["create_time", "key_id", "label", "sha256", "status"]);
      expect(k.sha256).toMatch(/^[0-9a-f]{64}$/);
    }
    expect(await readdir(dirname(keysFile))).toEqual(["keys.json"]);
  });

  it("serializes concurrent writes so no key is lost", async () => {
    const { app, keysFile } = await setup();
    const issued = await Promise.all(Array.from({ length: 10 }, (_, i) => generateKey(app, `sa_${i}`)));
    const { keys } = JSON.parse(await readFile(keysFile, "utf8")) as { keys: { key_id: string }[] };
    expect(keys.map((k) => k.key_id).sort()).toEqual(issued.map((k) => k.key_id).sort());
  });

  it("refuses to start on a damaged key file instead of overwriting it", async () => {
    const { keysFile } = await setup();
    await mkdir(dirname(keysFile), { recursive: true });
    await writeFile(keysFile, '{"keys":[{"key_id":"key_1"}]}');
    await expect(buildApp({ serviceCredential: SERVICE_CREDENTIAL, keysFile, log: memoryLogger() })).rejects.toThrow(KeyFileError);
    await writeFile(keysFile, "{");
    await expect(buildApp({ serviceCredential: SERVICE_CREDENTIAL, keysFile, log: memoryLogger() })).rejects.toThrow(KeyFileError);
  });
});
