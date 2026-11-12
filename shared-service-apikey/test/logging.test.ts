import { afterEach, describe, expect, it, vi } from "vitest";
import { generateKey, headers, SERVICE_CREDENTIAL, setup } from "./helpers.ts";

afterEach(() => {
  vi.restoreAllMocks();
});

// Captures what reaches stdout, stderr and console too, so a stray console.log in the code would be caught.
function captureProcessOutput(): string[] {
  const out: string[] = [];
  const grab = (chunk: unknown) => {
    out.push(String(chunk));
    return true;
  };
  vi.spyOn(process.stdout, "write").mockImplementation(grab);
  vi.spyOn(process.stderr, "write").mockImplementation(grab);
  for (const m of ["log", "info", "warn", "error", "debug"] as const) {
    vi.spyOn(console, m).mockImplementation((...args: unknown[]) => void out.push(args.map(String).join(" ")));
  }
  return out;
}

describe("logs", () => {
  it("never carry the service credential or a mirrored key", async () => {
    const env = await setup();
    const processOutput = captureProcessOutput();

    const generated = await generateKey(env.app, "sa_X");
    const rotateRes = await env.app.request(`/keys/${generated.key_id}/rotate`, {
      method: "POST",
      headers: headers(SERVICE_CREDENTIAL, null, null),
      body: JSON.stringify({ key_id: generated.key_id }),
    });
    const rotated = (await rotateRes.json()) as { key: string };
    const second = await generateKey(env.app, "sa_Y");

    await env.app.request("/shipments", { headers: headers(SERVICE_CREDENTIAL) });
    await env.app.request("/shipments", {
      method: "POST",
      headers: headers(rotated.key),
      body: JSON.stringify({ destination: "Makassar" }),
    });
    await env.app.request("/shipments", { headers: headers(generated.key) });
    await env.app.request("/keys", { method: "POST", headers: headers(second.key), body: "{}" });
    await env.app.request(`/keys/${second.key_id}`, { method: "DELETE", headers: headers(SERVICE_CREDENTIAL) });

    const logged = [...env.log.lines, ...processOutput].join("\n");
    expect(env.log.lines.some((l) => l.includes("key generated"))).toBe(true);
    expect(env.log.lines.some((l) => l.includes("key rotated"))).toBe(true);
    expect(env.log.lines.some((l) => l.includes("credential=mirrored"))).toBe(true);
    expect(env.log.lines.some((l) => l.includes("credential=service"))).toBe(true);

    for (const secret of [SERVICE_CREDENTIAL, generated.key, rotated.key, second.key]) {
      expect(logged).not.toContain(secret);
      expect(logged).not.toContain(secret.replace(/^pgk_/, ""));
    }
  });
});
