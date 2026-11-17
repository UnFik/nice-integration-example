import { describe, expect, it } from "vitest";
import { initials, relativeTime, shortId } from "./format";

const NOW = Date.parse("2026-11-16T12:00:00Z");
const at = (secondsAgo: number) => new Date(NOW - secondsAgo * 1000).toISOString();

describe("relativeTime", () => {
  it("uses the largest unit that fits, in Indonesian", () => {
    expect(relativeTime(at(10), NOW)).toBe("baru saja");
    expect(relativeTime(at(5 * 60), NOW)).toBe("5 menit yang lalu");
    expect(relativeTime(at(3 * 3600), NOW)).toBe("3 jam yang lalu");
    expect(relativeTime(at(3 * 86400), NOW)).toBe("3 hari yang lalu");
  });

  it("falls back to a date after a month and to a dash for nonsense", () => {
    expect(relativeTime(at(60 * 86400), NOW)).toMatch(/2026/);
    expect(relativeTime(undefined, NOW)).toBe("-");
    expect(relativeTime("kemarin", NOW)).toBe("-");
  });
});

describe("small formatters", () => {
  it("shortens long ids and leaves short ones", () => {
    expect(shortId("usr_01a17b49-56bc-7632-a32f-d47311c068c1")).toBe("usr_01a1…68c1");
    expect(shortId("usr_1")).toBe("usr_1");
    expect(shortId(undefined)).toBe("-");
  });

  it("takes up to two initials", () => {
    expect(initials("Siti Rahma Dewi")).toBe("SR");
    expect(initials("budi")).toBe("B");
    expect(initials(undefined)).toBe("?");
  });
});
