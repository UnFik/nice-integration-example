import { describe, expect, it } from "vitest";
import { RECENT_KEY, readRecent, remember, validateManualContext } from "./recent-contexts";
import { MemoryStorage } from "./test/memory-storage";

describe("recent contexts", () => {
  it("keeps the newest first without duplicates and at most five", () => {
    const storage = new MemoryStorage();
    for (const n of [1, 2, 3, 4, 5, 6]) remember(storage, { org: "org_a", project: `prj_${n}` });
    remember(storage, { org: "org_a", project: "prj_4" });

    expect(readRecent(storage).map((entry) => entry.project)).toEqual(["prj_4", "prj_6", "prj_5", "prj_3", "prj_2"]);
  });

  it("stores ids only", () => {
    const storage = new MemoryStorage();
    remember(storage, { org: "org_a", project: "prj_1" });

    expect(JSON.parse(storage.getItem(RECENT_KEY) ?? "")).toEqual([{ org: "org_a", project: "prj_1" }]);
  });

  it("reads a damaged value as empty", () => {
    const storage = new MemoryStorage();
    storage.setItem(RECENT_KEY, "{oops");
    expect(readRecent(storage)).toEqual([]);
    storage.setItem(RECENT_KEY, JSON.stringify([{ org: "org_a" }, 7, { org: "org_b", project: "prj_2" }]));
    expect(readRecent(storage)).toEqual([{ org: "org_b", project: "prj_2" }]);
  });
});

describe("validateManualContext", () => {
  it("trims both ids", () => {
    expect(validateManualContext(" org_a ", " prj_1 ")).toEqual({ org: "org_a", project: "prj_1" });
  });

  it.each([
    ["", "prj_1", "org"],
    ["org a", "prj_1", "org"],
    ["org_a", "", "project"],
    ["org_a", "proj_1", "project"],
    ["org_a", "prj_", "project"],
    ["org_a", "prj_1 2", "project"],
  ])("refuses %j and %j on %s", (org, project, field) => {
    expect(validateManualContext(org, project)).toMatchObject({ field });
  });
});
