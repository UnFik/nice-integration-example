import { describe, expect, it } from "vitest";
import { headers, SERVICE_CREDENTIAL, setup } from "./helpers.ts";

describe("X-Api-Key", () => {
  it("answers 401 without a key", async () => {
    const { app } = await setup();
    const res = await app.request("/shipments", { headers: headers(null) });
    expect(res.status).toBe(401);
    expect(await res.json()).toEqual({ error: "unauthorized" });
  });

  it("answers 401 for a wrong key, an empty key, and a Bearer value", async () => {
    const { app } = await setup();
    for (const key of ["wrong", "", `Bearer ${SERVICE_CREDENTIAL}`, SERVICE_CREDENTIAL + "x"]) {
      const res = await app.request("/shipments", { headers: headers(key) });
      expect(res.status, key).toBe(401);
      expect(await res.json()).toEqual({ error: "unauthorized" });
    }
  });

  it("does not process a request without a valid key", async () => {
    const { app } = await setup();
    const res = await app.request("/shipments", {
      method: "POST",
      headers: headers("wrong"),
      body: JSON.stringify({ destination: "Surabaya" }),
    });
    expect(res.status).toBe(401);
    const list = await app.request("/shipments", { headers: headers(SERVICE_CREDENTIAL) });
    expect(await list.json()).toEqual({ shipments: [] });
  });

  it("accepts the service credential", async () => {
    const { app } = await setup();
    const res = await app.request("/shipments", { headers: headers(SERVICE_CREDENTIAL) });
    expect(res.status).toBe(200);
  });

  it("guards unknown paths too", async () => {
    const { app } = await setup();
    expect((await app.request("/nope")).status).toBe(401);
    expect((await app.request("/nope", { headers: headers(SERVICE_CREDENTIAL) })).status).toBe(404);
  });

  it("leaves /healthz and /openapi.json open", async () => {
    const { app } = await setup();
    const health = await app.request("/healthz");
    expect(health.status).toBe(200);
    expect(await health.json()).toEqual({ status: "ok" });
    expect((await app.request("/openapi.json")).status).toBe(200);
  });
});
