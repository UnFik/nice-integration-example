import { describe, expect, it } from "vitest";
import { countdown, initials, relativeTime, shortId } from "./format";
import { ageRatio, tokenPhase } from "./token-status";

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

  it("counts down as m:ss and h:mm:ss", () => {
    expect(countdown(65_000)).toBe("1:05");
    expect(countdown(3_725_000)).toBe("1:02:05");
    expect(countdown(-5)).toBe("0:00");
  });
});

describe("token status", () => {
  const token = { issuedAt: NOW - 100_000, expiresAt: NOW + 900_000, refreshAt: NOW + 300_000 };

  it("is active, then soon at refreshAt, then expired", () => {
    expect(tokenPhase(token, NOW)).toBe("active");
    expect(tokenPhase(token, NOW + 300_000)).toBe("soon");
    expect(tokenPhase(token, NOW + 900_000)).toBe("expired");
  });

  it("turns soon a minute before exp even when refreshAt is later", () => {
    expect(tokenPhase({ ...token, refreshAt: NOW + 2_000_000 }, NOW + 850_000)).toBe("soon");
  });

  it("measures the age between 0 and 1", () => {
    expect(ageRatio(token, NOW)).toBeCloseTo(0.1);
    expect(ageRatio(token, NOW + 5_000_000)).toBe(1);
    expect(ageRatio(token, NOW - 5_000_000)).toBe(0);
  });
});
