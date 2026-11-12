import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { ConfigError, loadConfig } from "../src/config.ts";

const root = fileURLToPath(new URL("..", import.meta.url));

describe("config", () => {
  it("requires SERVICE_CREDENTIAL and names it", () => {
    expect(() => loadConfig({})).toThrow(ConfigError);
    expect(() => loadConfig({})).toThrow(/SERVICE_CREDENTIAL/);
    expect(() => loadConfig({ SERVICE_CREDENTIAL: "   " })).toThrow(/SERVICE_CREDENTIAL/);
  });

  it("uses the defaults", () => {
    expect(loadConfig({ SERVICE_CREDENTIAL: "c" })).toEqual({
      serviceCredential: "c",
      host: "127.0.0.1",
      port: 8082,
      keysFile: "data/keys.json",
    });
  });

  it("reads LISTEN_ADDR and KEYS_FILE", () => {
    const c = loadConfig({ SERVICE_CREDENTIAL: "c", LISTEN_ADDR: "[::1]:9000", KEYS_FILE: "/tmp/k.json" });
    expect(c).toMatchObject({ host: "::1", port: 9000, keysFile: "/tmp/k.json" });
  });

  it("names LISTEN_ADDR when it is malformed", () => {
    for (const bad of ["8082", "127.0.0.1:", "127.0.0.1:0", "127.0.0.1:70000", ":8082", "host:abc"]) {
      expect(() => loadConfig({ SERVICE_CREDENTIAL: "c", LISTEN_ADDR: bad })).toThrow(/LISTEN_ADDR/);
    }
  });

  it("stops the process at start without SERVICE_CREDENTIAL", () => {
    const env: Record<string, string> = { PATH: process.env.PATH ?? "" };
    const out = spawnSync(process.execPath, ["src/index.ts"], { cwd: root, env, encoding: "utf8", timeout: 20_000 });
    expect(out.status).toBe(1);
    expect(out.stderr).toContain("SERVICE_CREDENTIAL");
  });
});
