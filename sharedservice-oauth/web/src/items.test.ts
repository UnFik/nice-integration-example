import { describe, expect, it } from "vitest";
import { ApiError } from "./api";
import { describeItemsError, validateItem } from "./items";

describe("describeItemsError", () => {
  it("explains a revoked access on a sensitive operation", () => {
    const message = describeItemsError(new ApiError(403, "access_denied"), "delete");

    expect(message).toMatch(/dicabut/);
    expect(message).toMatch(/menghapus barang/);
  });

  it("tells a missing permission from a revoked access", () => {
    expect(describeItemsError(new ApiError(403, "permission_denied"), "edit")).toMatch(/tidak punya izin untuk mengubah barang/);
  });

  it("names the field of a rejected input and the retry for a failed access check", () => {
    expect(describeItemsError(new ApiError(400, "invalid_argument", "name"), "create")).toContain("(name)");
    expect(describeItemsError(new ApiError(503, "unavailable"), "delete")).toMatch(/Coba lagi/);
  });

  it("points at the backend when the call never got an answer", () => {
    expect(describeItemsError(new TypeError("Failed to fetch"), "read")).toMatch(/Backend inventaris tidak dapat dipanggil/);
  });

  it("explains a missing or refused context", () => {
    expect(describeItemsError(new ApiError(409, "context_required"), "read")).toMatch(/Pilih project/);
    expect(describeItemsError(new ApiError(409, "context_refused", "access_denied"), "read")).toMatch(/\(access_denied\)/);
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
