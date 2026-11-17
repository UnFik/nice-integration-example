import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { KEY_ROUTES } from "../src/app.ts";

const read = (name: string) => readFileSync(fileURLToPath(new URL(`../${name}`, import.meta.url)), "utf8");

const readmes = [
  { file: "README.md", config: "## Konfigurasi" },
  { file: "README.en.md", config: "## Configuration" },
];

const envExampleNames = [...read(".env.example").matchAll(/^([A-Z][A-Z0-9_]*)=/gm)].map((m) => m[1]).sort();

function section(text: string, heading: string): string {
  const start = text.indexOf(`\n${heading}\n`);
  expect(start, heading).toBeGreaterThan(-1);
  const end = text.indexOf("\n## ", start + heading.length + 2);
  return text.slice(start, end < 0 ? undefined : end);
}

const stepHeadings = (text: string) => [...text.matchAll(/^### (\d+)\. /gm)].map((m) => Number(m[1]));

describe.each(readmes)("$file", ({ file, config }) => {
  const text = read(file);

  it("documents exactly the variables of .env.example", () => {
    const names = [...section(text, config).matchAll(/^\| `([A-Z][A-Z0-9_]*)` \|/gm)].map((m) => m[1]).sort();
    expect(names).toEqual(envExampleNames);
  });

  it("registers the key operation paths the code serves", () => {
    const start = text.indexOf("\n### 4. ");
    const registration = text.slice(start, text.indexOf("\n### 5. ", start));
    for (const [name, { method, path }] of Object.entries(KEY_ROUTES)) {
      expect(registration).toContain(`${name}: {method: "${method}", path: "${path}"}`);
    }
    for (const { method, path } of Object.values(KEY_ROUTES)) {
      expect(text).toContain(`| \`${method} ${path}`);
    }
  });

  it("reads the registration and activation answers inside shared_service", () => {
    // NICE answers both with {"shared_service": {...}}; a filter on top-level fields prints only nulls.
    for (const step of [4, 6]) {
      const start = text.indexOf(`\n### ${step}. `);
      const body = text.slice(start, text.indexOf("\n### ", start + 1));
      const filters = [...body.matchAll(/\| jq '([^']*)'/g)].map((m) => m[1]);
      expect(filters, `step ${step}`).not.toHaveLength(0);
      for (const filter of filters) expect(filter, `step ${step}`).toMatch(/^\.shared_service \|/);
    }
  });

  it("numbers the checklist 1 to 12 with no gaps", () => {
    expect(stepHeadings(text)).toEqual(Array.from({ length: 12 }, (_, i) => i + 1));
  });
});

describe("scripts/smoke.sh", () => {
  const smoke = read("scripts/smoke.sh");

  it("points only at checklist steps that exist", () => {
    const refs = new Set<number>();
    for (const m of smoke.matchAll(/fail "(?:[^"\\]|\\.)*" "?([\d, -]+)"?[ \t]*(?:;|$)/gm)) {
      for (const part of m[1]!.split(",")) {
        const [a, b] = part.trim().split("-").map(Number);
        for (let n = a!; n <= (b ?? a!); n++) refs.add(n);
      }
    }
    expect(refs.size).toBeGreaterThan(5);
    const steps = new Set(stepHeadings(read("README.md")));
    for (const n of refs) expect(steps.has(n), `step ${n}`).toBe(true);
  });

  it("reads only variables that .env.example declares", () => {
    const used = new Set([...smoke.matchAll(/\$\{?([A-Z][A-Z0-9_]*)/g)].map((m) => m[1]));
    for (const name of used) expect(envExampleNames, name).toContain(name);
  });
});
