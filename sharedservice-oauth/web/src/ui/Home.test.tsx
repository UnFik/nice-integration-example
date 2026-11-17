import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { Contexts } from "../api";
import type { Item } from "../items";
import { Session, type SessionState } from "../session";
import { FULL, ORG_A, ORG_B, PRJ_P } from "../test/fake-backend";
import { Home } from "./Home";

const session = new Session({ fetch: () => Promise.reject(new Error("offline")), navigate: () => {}, replaceUrl: () => {} });

const contexts: Contexts = {
  currentOrganizationId: ORG_A,
  organizations: [
    { id: ORG_A, name: "Nusapala", slug: "nusapala", projects: [{ id: PRJ_P, name: "Parkir", slug: "parkir" }], projectsTruncated: false },
    { id: ORG_B, name: "Bintang Laut", slug: "bintang-laut", projects: [], projectsTruncated: false },
  ],
};

const item: Item = { id: "itm-1", name: "Kabel UTP Cat6", quantity: 12, createdBy: "usr_01a17b49-56bc-7632-a32f-d47311c068c1", createTime: "2026-11-01T10:00:00Z", updateTime: "2026-11-02T10:00:00Z" };

const signedIn: SessionState = {
  phase: "signed-in",
  user: { id: "usr_1", name: "Siti Rahma", email: "siti@nusapala.id" },
  active: { org: ORG_A, project: PRJ_P },
  perms: FULL,
  contexts,
  items: [item],
};

const noContext: SessionState = { ...signedIn, active: undefined, perms: [], items: undefined };

const render = (state: SessionState) => renderToStaticMarkup(<Home session={session} state={state} />);
const disabled = (markup: string, action: string) => new RegExp(`<button[^>]*data-action="${action}"[^>]*disabled=""|<button[^>]*disabled=""[^>]*data-action="${action}"`).test(markup);

describe("Home", () => {
  it("shows the identity and the permissions from /auth/session", () => {
    const markup = render(signedIn);

    expect(markup).toMatch(/<dt>Email<\/dt><dd>siti@nusapala.id<\/dd>/);
    expect(markup).toContain("<code>inventaris.items.delete</code>");
  });

  it("shows the active context as a breadcrumb with names from /auth/contexts", () => {
    const markup = render(signedIn);

    expect(markup).toMatch(/Organisasi<\/span> <strong>Nusapala<\/strong>/);
    expect(markup).toMatch(/Project<\/span> <strong>Parkir<\/strong>/);
  });

  it("falls back to short ids while the choices are not loaded", () => {
    expect(render({ ...signedIn, contexts: undefined })).toMatch(/Project<\/span> <strong>prj_0000…0001<\/strong>/);
  });

  it("asks to choose a project when there is no context, with the picker", () => {
    const markup = render(noContext);

    expect(markup).toMatch(/Belum ada konteks/);
    expect(markup).toContain('href="#konteks"');
    expect(markup).toContain(`data-context="${ORG_A}/${PRJ_P}"`);
    expect(markup).toMatch(/Organisasi saat ini/);
    expect(markup).not.toMatch(/data-action="add"/);
  });

  it("shows an organization without projects as unavailable", () => {
    expect(render(signedIn)).toContain(`data-unavailable="${ORG_B}"`);
  });

  it("shows a refused switch and keeps the picker", () => {
    const markup = render({ ...signedIn, switchError: "Konteks itu tidak dapat dipakai, Anda tetap di konteks sebelumnya. NICE menolak (invalid_scope)." });

    expect(markup).toMatch(/callout warning/);
    expect(markup).toContain("tetap di konteks sebelumnya");
    expect(markup).toContain(`data-context="${ORG_A}/${PRJ_P}"`);
  });

  it("shows why the token of the active context could not be had, and lists nothing", () => {
    const markup = render({ ...signedIn, contextError: "access_denied", perms: [], items: undefined });

    expect(markup).toContain('data-context-error="access_denied"');
    expect(markup).toMatch(/tidak boleh memakai inventaris di organisasi itu/);
    expect(markup).toMatch(/Konteks aktif tidak dapat dipakai/);
    expect(markup).not.toMatch(/Memuat barang/);
  });

  it("shows a failed discovery and keeps the items", () => {
    const markup = render({ ...signedIn, contexts: undefined, contextsError: "Pilihan konteks tidak dapat dimuat: NICE menolak penemuan konteks (invalid_grant). Konteks aktif tetap dipakai." });

    expect(markup).toContain("NICE menolak penemuan konteks (invalid_grant)");
    expect(markup).toContain("Kabel UTP Cat6");
    expect(markup).not.toMatch(/Memuat konteks/);
  });

  it("only says the choices are loading while no answer or error exists", () => {
    expect(render(signedIn)).not.toMatch(/Memuat konteks/);
    expect(render({ ...signedIn, contexts: undefined })).toMatch(/Memuat konteks…/);
  });

  it("offers to retry a sign-out that NICE did not answer", () => {
    const markup = render({ ...signedIn, logoutError: "NICE tidak menjawab, jadi refresh token belum dicabut dan Anda belum keluar. Sesi tetap aktif." });

    expect(markup).toMatch(/role="alert"[^>]*>.*Anda belum keluar/);
    expect(markup).toMatch(/Coba lagi/);
  });

  it("disables Keluar while signing out", () => {
    expect(render({ ...signedIn, loggingOut: true })).toMatch(/<button type="button" disabled="">.*Keluar…/);
  });
});

describe("items", () => {
  it("lists the items with a short creator, a relative time and one row per item", () => {
    const markup = render(signedIn);

    expect(markup).toContain("Kabel UTP Cat6");
    expect(markup).toContain("usr_01a1…68c1");
    expect(markup).toMatch(/<time dateTime="2026-11-02T10:00:00Z"/);
    expect(markup).toMatch(/1 barang/);
    expect(markup.match(/class="item-row"/g)).toHaveLength(1);
  });

  it("enables every action when perms carries all four permissions", () => {
    const markup = render(signedIn);

    expect(disabled(markup, "add")).toBe(false);
    expect(disabled(markup, "edit")).toBe(false);
    expect(disabled(markup, "delete")).toBe(false);
    expect(markup).not.toMatch(/dinonaktifkan/);
  });

  it("disables what perms does not allow and says which permission is missing", () => {
    const markup = render({ ...signedIn, perms: ["inventaris.items.read", "inventaris.items.edit"] });

    expect(disabled(markup, "add")).toBe(true);
    expect(disabled(markup, "edit")).toBe(false);
    expect(disabled(markup, "delete")).toBe(true);
    expect(markup).toMatch(/Tambah, Hapus dinonaktifkan: token konteks ini tidak memuat inventaris.items.create, inventaris.items.delete/);
  });

  it("has its own loading, error and empty views", () => {
    expect(render({ ...signedIn, items: undefined })).toMatch(/Memuat barang…/);
    const failed = render({ ...signedIn, items: undefined, itemsError: "inventaris belum bisa memeriksa akses ke NICE." });
    expect(failed).toMatch(/Barang tidak dapat dimuat/);
    expect(failed).toMatch(/Coba lagi/);
    const empty = render({ ...signedIn, items: [] });
    expect(empty).toMatch(/Belum ada barang di project ini/);
    expect(empty).toMatch(/Tambah barang/);
  });

  it("does not offer to add the first item without the permission", () => {
    expect(render({ ...signedIn, items: [], perms: ["inventaris.items.read"] })).toMatch(/Token Anda tidak boleh menambah barang/);
  });
});
