import { describe, expect, it } from "vitest";
import { setup } from "./helpers.ts";

const METHODS = ["get", "put", "post", "delete", "options", "head", "patch", "trace"];

type Doc = { openapi: string; paths: Record<string, Record<string, Record<string, unknown>>> };

describe("/openapi.json", () => {
  it("holds exactly the three shipment operations with their permissions", async () => {
    const { app } = await setup();
    const res = await app.request("/openapi.json");
    expect(res.status).toBe(200);
    const doc = (await res.json()) as Doc;
    expect(doc.openapi).toMatch(/^3\./);

    const ops = Object.entries(doc.paths).flatMap(([path, item]) =>
      Object.entries(item)
        .filter(([method]) => METHODS.includes(method))
        .map(([method, op]) => ({
          route: `${method.toUpperCase()} ${path}`,
          operationId: op.operationId,
          permission: op["x-nice-permission"],
        })),
    );
    expect(ops).toEqual([
      { route: "GET /shipments", operationId: "listShipments", permission: "pengiriman.shipments.read" },
      { route: "POST /shipments", operationId: "createShipment", permission: "pengiriman.shipments.create" },
      { route: "POST /shipments/{id}/cancel", operationId: "cancelShipment", permission: "pengiriman.shipments.cancel" },
    ]);
  });

  it("leaves out the key operations, /healthz and /openapi.json", async () => {
    const { app } = await setup();
    const doc = (await (await app.request("/openapi.json")).json()) as Doc;
    for (const path of Object.keys(doc.paths)) {
      expect(path).not.toMatch(/^\/(keys|healthz|openapi\.json)/);
    }
  });

  it("uses internal $ref only, since the NICE importer refuses external ones", async () => {
    const { app } = await setup();
    const text = await (await app.request("/openapi.json")).text();
    for (const [, ref] of text.matchAll(/"\$ref":"([^"]*)"/g)) {
      expect(ref).toMatch(/^#\//);
    }
  });
});
