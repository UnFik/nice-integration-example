import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { Contexts } from "../contexts";
import { ContextPicker } from "./ContextPicker";

const contexts: Contexts = {
  currentOrganizationId: "org-a",
  organizations: [
    {
      id: "org-a",
      name: "Nusapala",
      slug: "nusapala",
      projects: [
        { id: "prj-p", name: "Parkir", slug: "parkir", services: ["inventaris"] },
        { id: "prj-q", name: "Gudang", slug: "gudang", services: ["inventaris", "pengiriman"] },
      ],
      projectsTruncated: false,
    },
    {
      id: "org-b",
      name: "Bintang Laut",
      slug: "bintang-laut",
      projects: [{ id: "prj-r", name: "Retail", slug: "retail", services: ["inventaris"] }],
      projectsTruncated: true,
    },
    { id: "org-c", name: "Cakrawala", slug: "cakrawala", projects: [], projectsTruncated: false },
  ],
};

function render(active: { org: string; project?: string }, busy = false): string {
  return renderToStaticMarkup(<ContextPicker contexts={contexts} active={active} busy={busy} onChoose={() => {}} />);
}

const choices = (markup: string) => [...markup.matchAll(/data-context="([^"]+)"/g)].map((match) => match[1]);
const current = (markup: string) => [...markup.matchAll(/data-context="([^"]+)"[^>]*aria-current="true"/g)].map((m) => m[1]);

describe("ContextPicker", () => {
  it("offers exactly the projects of the answer and marks the active one", () => {
    const markup = render({ org: "org-a", project: "prj-p" });

    expect(choices(markup)).toEqual(["org-a/prj-p", "org-a/prj-q", "org-b/prj-r"]);
    expect(current(markup)).toEqual(["org-a/prj-p"]);
    expect(markup).toContain("Parkir");
    expect(markup).toContain("pengiriman");
  });

  it("shows an organization without projects as unavailable with the reason", () => {
    const markup = render({ org: "org-a", project: "prj-p" });
    const section = markup.slice(markup.indexOf("Cakrawala"));

    expect(section).toMatch(/Tidak tersedia/);
    expect(section).toMatch(/tidak memegang akses inventaris di project mana pun/);
  });

  it("marks the current organization before a project is chosen", () => {
    const markup = render({ org: "org-b" });

    expect(current(markup)).toEqual([]);
    expect(markup).toMatch(/Bintang Laut[^]*?Organisasi saat ini/);
    expect(markup.match(/Organisasi saat ini/g)).toHaveLength(1);
  });

  it("says when NICE left projects out", () => {
    expect(render({ org: "org-a" })).toMatch(/hanya mengirim 100 project pertama/);
  });

  it("disables every choice while a switch runs", () => {
    const markup = render({ org: "org-a", project: "prj-p" }, true);

    expect(markup.match(/<button[^>]*disabled=""/g)).toHaveLength(3);
  });
});
