import { describe, expect, it } from "vitest";
import { describeItemsError, itemsApi, ItemsApiError, permissionsOf, toItem, validateItem } from "./items";
import { json } from "./test/fake-nice";
import type { TokenKey } from "./token-manager";

const target: TokenKey = { org: "org_a", service: "inventaris", project: "prj_p" };

function client(answer: (url: string, init: RequestInit) => Response) {
  const calls: { service: TokenKey; url: string; init: RequestInit }[] = [];
  const tokens = {
    call: async (service: TokenKey, url: string, init: RequestInit = {}) => {
      calls.push({ service, url, init });
      return answer(url, init);
    },
  };
  return { api: itemsApi(tokens, "http://127.0.0.1:8081", target), calls };
}

const stored = { id: "itm_1", name: "Kabel", quantity: 3, created_by: "usr_1", create_time: "2026-11-01T10:00:00Z", update_time: "2026-11-02T10:00:00Z" };

describe("itemsApi", () => {
  it("lists items through tokens.call with the token of the context", async () => {
    const { api, calls } = client(() => json(200, { items: [stored] }));

    const items = await api.list();

    expect(items).toEqual([{ id: "itm_1", name: "Kabel", quantity: 3, createdBy: "usr_1", createTime: "2026-11-01T10:00:00Z", updateTime: "2026-11-02T10:00:00Z" }]);
    expect(calls[0]).toMatchObject({ service: target, url: "http://127.0.0.1:8081/items", init: { method: "GET" } });
  });

  it("creates with a JSON body and updates with PUT on the encoded id", async () => {
    const { api, calls } = client(() => json(200, stored));

    await api.create({ name: "Kabel", quantity: 3 });
    await api.update("itm/1", { name: "Kabel", quantity: 4 });

    expect(calls[0].init).toMatchObject({ method: "POST", body: '{"name":"Kabel","quantity":3}' });
    expect(new Headers(calls[0].init.headers).get("content-type")).toBe("application/json");
    expect(calls[1].url).toBe("http://127.0.0.1:8081/items/itm%2F1");
    expect(calls[1].init).toMatchObject({ method: "PUT", body: '{"name":"Kabel","quantity":4}' });
  });

  it("deletes and accepts 204 without a body", async () => {
    const { api, calls } = client(() => new Response(null, { status: 204 }));

    await expect(api.remove("itm_1")).resolves.toBeUndefined();
    expect(calls[0]).toMatchObject({ url: "http://127.0.0.1:8081/items/itm_1", init: { method: "DELETE" } });
  });

  it("raises the error and description of inventaris", async () => {
    const { api } = client(() => json(400, { error: "invalid_argument", error_description: "name" }));

    await expect(api.create({ name: "", quantity: 1 })).rejects.toMatchObject({ status: 400, code: "invalid_argument", description: "name" });
  });

  it("rejects a list answer without items", async () => {
    const { api } = client(() => json(200, {}));

    await expect(api.list()).rejects.toBeInstanceOf(ItemsApiError);
  });
});

describe("describeItemsError", () => {
  it("explains a revoked access on a sensitive operation", () => {
    const message = describeItemsError(new ItemsApiError(403, "access_denied"), "delete");

    expect(message).toMatch(/dicabut/);
    expect(message).toMatch(/menghapus barang/);
  });

  it("tells a missing permission from a revoked access", () => {
    expect(describeItemsError(new ItemsApiError(403, "permission_denied"), "edit")).toMatch(/tidak punya izin untuk mengubah barang/);
  });

  it("names the field of a rejected input and the retry for a failed access check", () => {
    expect(describeItemsError(new ItemsApiError(400, "invalid_argument", "name"), "create")).toContain("(name)");
    expect(describeItemsError(new ItemsApiError(503, "unavailable"), "delete")).toMatch(/Coba lagi/);
  });

  it("points at the service and CORS when the call never got an answer", () => {
    expect(describeItemsError(new TypeError("Failed to fetch"), "read")).toMatch(/CORS/);
  });
});

describe("validateItem", () => {
  it("trims the name and reads the quantity as an integer", () => {
    expect(validateItem("  Kabel  ", " 12 ")).toEqual({ input: { name: "Kabel", quantity: 12 } });
  });

  it.each([
    ["", "1", "name"],
    ["   ", "1", "name"],
    ["x".repeat(121), "1", "name"],
    ["Kabel", "", "quantity"],
    ["Kabel", "-1", "quantity"],
    ["Kabel", "1.5", "quantity"],
    ["Kabel", "abc", "quantity"],
  ])("refuses %j with quantity %j on %s", (name, quantity, field) => {
    expect(validateItem(name, quantity)).toMatchObject({ field });
  });
});

describe("permissionsOf and toItem", () => {
  it("reads perms and keeps only strings", () => {
    expect(permissionsOf({ perms: ["inventaris.items.read", 3] })).toEqual(["inventaris.items.read"]);
    expect(permissionsOf({})).toBeUndefined();
    expect(permissionsOf(undefined)).toBeUndefined();
  });

  it("fills what an answer leaves out", () => {
    expect(toItem({ id: "itm_1" })).toEqual({ id: "itm_1", name: "", quantity: 0, createdBy: "", createTime: undefined, updateTime: undefined });
  });
});
