import { describe, expect, it } from "vitest";
import { generateKey, headers, SERVICE_CREDENTIAL, setup } from "./helpers.ts";

type Shipment = { id: string; destination: string; status: string; created_by: string | null; create_time: string };

const as = (org: string | null, principal: string | null = "sa_X") => headers(SERVICE_CREDENTIAL, org, principal);

async function create(app: Awaited<ReturnType<typeof setup>>["app"], org: string, destination = "Gudang Timur, Surabaya", principal = "sa_X") {
  const res = await app.request("/shipments", { method: "POST", headers: as(org, principal), body: JSON.stringify({ destination }) });
  return { res, body: (await res.json()) as Shipment };
}

describe("shipments", () => {
  it("creates a shipment with status created for the organization", async () => {
    const { app } = await setup();
    const { res, body } = await create(app, "org_A");
    expect(res.status).toBe(201);
    expect(body).toMatchObject({ destination: "Gudang Timur, Surabaya", status: "created", created_by: "sa_X" });
    expect(body.id).toMatch(/^shp_/);
    const list = (await (await app.request("/shipments", { headers: as("org_A") })).json()) as { shipments: Shipment[] };
    expect(list.shipments).toEqual([body]);
  });

  it("records created_by from X-Nice-Principal", async () => {
    const { app } = await setup();
    expect((await create(app, "org_A", "Medan", "usr_42")).body.created_by).toBe("usr_42");
    const res = await app.request("/shipments", { method: "POST", headers: as("org_A", null), body: JSON.stringify({ destination: "Medan" }) });
    expect(((await res.json()) as Shipment).created_by).toBeNull();
  });

  it("keeps organizations apart", async () => {
    const { app } = await setup();
    const a = (await create(app, "org_A")).body;
    const listB = (await (await app.request("/shipments", { headers: as("org_B") })).json()) as { shipments: Shipment[] };
    expect(listB.shipments).toEqual([]);
    const cancelFromB = await app.request(`/shipments/${a.id}/cancel`, { method: "POST", headers: as("org_B") });
    expect(cancelFromB.status).toBe(404);
    const listA = (await (await app.request("/shipments", { headers: as("org_A") })).json()) as { shipments: Shipment[] };
    expect(listA.shipments[0]?.status).toBe("created");
  });

  it("answers 400 missing_organization on every shipment operation", async () => {
    const { app } = await setup();
    const calls: [string, string, string?][] = [
      ["GET", "/shipments"],
      ["POST", "/shipments", JSON.stringify({ destination: "Medan" })],
      ["POST", "/shipments/shp_x/cancel"],
    ];
    for (const [method, path, body] of calls) {
      for (const org of [null, "  "]) {
        const res = await app.request(path, { method, headers: as(org), body });
        expect(res.status, `${method} ${path}`).toBe(400);
        expect(await res.json()).toEqual({ error: "missing_organization" });
      }
    }
  });

  it("checks the credential before the organization", async () => {
    const { app } = await setup();
    const res = await app.request("/shipments", { headers: headers(null, null) });
    expect(res.status).toBe(401);
  });

  it("rejects an empty, blank, or too long destination with invalid_argument", async () => {
    const { app } = await setup();
    for (const destination of ["", "   ", "x".repeat(201)]) {
      const res = await app.request("/shipments", { method: "POST", headers: as("org_A"), body: JSON.stringify({ destination }) });
      expect(res.status, JSON.stringify(destination)).toBe(400);
      expect(await res.json()).toEqual({ error: "invalid_argument", error_description: "destination" });
    }
    const ok = await create(app, "org_A", "x".repeat(200));
    expect(ok.res.status).toBe(201);
  });

  it("rejects a missing destination, a missing body, and malformed JSON", async () => {
    const { app } = await setup();
    const missing = await app.request("/shipments", { method: "POST", headers: as("org_A"), body: "{}" });
    expect(missing.status).toBe(400);
    expect(await missing.json()).toEqual({ error: "invalid_argument", error_description: "destination" });

    const noBody = await app.request("/shipments", { method: "POST", headers: as("org_A") });
    expect(noBody.status).toBe(400);
    expect(((await noBody.json()) as { error: string }).error).toBe("invalid_argument");

    const malformed = await app.request("/shipments", { method: "POST", headers: as("org_A"), body: "{" });
    expect(malformed.status).toBe(400);
    expect(await malformed.json()).toEqual({ error: "invalid_argument", error_description: "body" });

    for (const body of ["[]", "null"]) {
      const res = await app.request("/shipments", { method: "POST", headers: as("org_A"), body });
      expect(res.status, body).toBe(400);
      expect(await res.json()).toEqual({ error: "invalid_argument", error_description: "body" });
    }

    const list = (await (await app.request("/shipments", { headers: as("org_A") })).json()) as { shipments: Shipment[] };
    expect(list.shipments).toEqual([]);
  });

  it("answers 415 for a body that is not declared as JSON", async () => {
    const { app } = await setup();
    const res = await app.request("/shipments", {
      method: "POST",
      headers: { ...as("org_A"), "Content-Type": "text/plain" },
      body: JSON.stringify({ destination: "Medan" }),
    });
    expect(res.status).toBe(415);
    expect(((await res.json()) as { error: string }).error).toBe("unsupported_media_type");
  });

  it("cancels once, then answers 409 already_cancelled", async () => {
    const { app } = await setup();
    const s = (await create(app, "org_A")).body;
    const first = await app.request(`/shipments/${s.id}/cancel`, { method: "POST", headers: as("org_A") });
    expect(first.status).toBe(200);
    expect(((await first.json()) as Shipment).status).toBe("cancelled");
    const second = await app.request(`/shipments/${s.id}/cancel`, { method: "POST", headers: as("org_A") });
    expect(second.status).toBe(409);
    expect(await second.json()).toEqual({ error: "already_cancelled" });
  });

  it("answers 404 not_found for an unknown shipment", async () => {
    const { app } = await setup();
    const res = await app.request("/shipments/shp_missing/cancel", { method: "POST", headers: as("org_A") });
    expect(res.status).toBe(404);
    expect(await res.json()).toEqual({ error: "not_found" });
  });

  it("serves shipments to a mirrored key as well", async () => {
    const { app } = await setup();
    const k = await generateKey(app);
    const res = await app.request("/shipments", { method: "POST", headers: headers(k.key, "org_A", "sa_X"), body: JSON.stringify({ destination: "Bali" }) });
    expect(res.status).toBe(201);
  });
});
