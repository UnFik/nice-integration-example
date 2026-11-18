import { describe, expect, it } from "vitest";
import { ApiError } from "./api";
import { describeDevicesError, validateDevice } from "./devices";

describe("describeDevicesError", () => {
  it("explains a revoked access on a sensitive operation", () => {
    const message = describeDevicesError(new ApiError(403, "access_denied"), "delete");

    expect(message).toMatch(/dicabut/);
    expect(message).toMatch(/menghapus perangkat/);
  });

  it("tells a missing permission from a revoked access", () => {
    expect(describeDevicesError(new ApiError(403, "permission_denied"), "edit")).toMatch(/tidak punya izin untuk mengubah perangkat/);
  });

  it("names the field of a rejected input and the retry for a failed access check", () => {
    expect(describeDevicesError(new ApiError(400, "invalid_argument", "name"), "create")).toContain("(name)");
    expect(describeDevicesError(new ApiError(503, "unavailable"), "delete")).toMatch(/Coba lagi/);
  });

  it("points at the backend when the call never got an answer", () => {
    expect(describeDevicesError(new TypeError("Failed to fetch"), "read")).toMatch(/Backend nadi tidak dapat dipanggil/);
  });

  it("explains a missing or refused context", () => {
    expect(describeDevicesError(new ApiError(409, "context_required"), "read")).toMatch(/Pilih project/);
    expect(describeDevicesError(new ApiError(409, "context_refused", "access_denied"), "read")).toMatch(/\(access_denied\)/);
  });
});

describe("validateDevice", () => {
  const valid = {
    name: "Sensor",
    serialNumber: "SN-1",
    status: "active",
    location: "",
  };

  it("trims every text field and keeps the location optional", () => {
    expect(
      validateDevice({
        name: "  Sensor  ",
        serialNumber: " SN-1 ",
        status: "maintenance",
        location: "  Gudang A ",
      }),
    ).toEqual({
      input: {
        name: "Sensor",
        serialNumber: "SN-1",
        status: "maintenance",
        location: "Gudang A",
      },
    });
    expect(validateDevice(valid)).toEqual({
      input: {
        name: "Sensor",
        serialNumber: "SN-1",
        status: "active",
        location: "",
      },
    });
  });

  it.each([
    ["an empty name", { name: "" }, "name"],
    ["a blank name", { name: "   " }, "name"],
    ["a name of 121 characters", { name: "x".repeat(121) }, "name"],
    ["a missing serial number", { serialNumber: " " }, "serialNumber"],
    ["a serial number of 65 characters", { serialNumber: "9".repeat(65) }, "serialNumber"],
    ["an unknown status", { status: "broken" }, "status"],
    ["an empty status", { status: "" }, "status"],
    ["a location of 121 characters", { location: "x".repeat(121) }, "location"],
  ])("refuses %s", (_label, change, field) => {
    expect(validateDevice({ ...valid, ...change })).toMatchObject({ field });
  });
});
