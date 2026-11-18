import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { ActiveContext, Contexts } from "../api";
import { ContextPicker } from "./ContextPicker";

const contexts: Contexts = {
  currentOrganizationId: "org-a",
  organizations: [
    {
      id: "org-a",
      name: "Nusapala",
      slug: "nusapala",
      projects: [
        { id: "prj-p", name: "Parkir", slug: "parkir" },
        { id: "prj-q", name: "Gudang", slug: "gudang" },
      ],
      projectsTruncated: false,
    },
    { id: "org-b", name: "Bintang Laut", slug: "bintang-laut", projects: [{ id: "prj-r", name: "Retail", slug: "retail" }], projectsTruncated: true },
    { id: "org-c", name: "Cakrawala", slug: "cakrawala", projects: [], projectsTruncated: false },
  ],
};

function render(active: ActiveContext | undefined, busy = false): string {
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
  });

  it("marks a project of another organization as active after a switch", () => {
    expect(current(render({ org: "org-b", project: "prj-r" }))).toEqual(["org-b/prj-r"]);
  });

  it("shows an organization without projects as unavailable with the reason", () => {
    const markup = render({ org: "org-a", project: "prj-p" });
    const section = markup.slice(markup.indexOf("Cakrawala"));

    expect(section).toContain('data-unavailable="org-c"');
    expect(section).toMatch(/Tidak tersedia: Anda tidak punya akses nadi di project mana pun/);
  });

  it("marks the organization of the login before a project is chosen", () => {
    const markup = render(undefined);

    expect(current(markup)).toEqual([]);
    expect(markup).toMatch(/Nusapala[^]*?Organisasi saat ini/);
    expect(markup.match(/Organisasi saat ini/g)).toHaveLength(1);
  });

  it("says when NICE left projects out", () => {
    expect(render(undefined)).toMatch(/tidak mengirim semua project/);
  });

  it("disables every choice while a switch runs", () => {
    expect(render({ org: "org-a", project: "prj-p" }, true).match(/<button[^>]*disabled=""/g)).toHaveLength(3);
  });
});
