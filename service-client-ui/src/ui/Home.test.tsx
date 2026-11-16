import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { Item } from "../items";
import { Session, type SessionState } from "../session";
import { CLIENT_ID, fakeJwt, INVENTARIS_URL, ORG_A, POST_LOGOUT_URI, PRJ_P, REDIRECT_URI, SUBJECT } from "../test/fake-nice";
import { MemoryStorage } from "../test/memory-storage";
import type { TokenView } from "../token-manager";
import { Home } from "./Home";

const session = new Session(
  { issuer: "https://nice.test", clientId: CLIENT_ID, redirectUri: REDIRECT_URI, postLogoutRedirectUri: POST_LOGOUT_URI, inventarisUrl: INVENTARIS_URL },
  { storage: new MemoryStorage(), fetch: () => Promise.reject(new Error("offline")), navigate: () => {}, replaceUrl: () => {} },
);

const FULL = ["inventaris.items.read", "inventaris.items.create", "inventaris.items.edit", "inventaris.items.delete"];

function inventarisToken(perms: string[], now = Date.now()): TokenView {
  const claims = { aud: "inventaris", prj: PRJ_P, nice_org_id: ORG_A, perms, iat: Math.floor(now / 1000), exp: Math.floor(now / 1000) + 1800 };
  return {
    key: `${ORG_A}|inventaris|${PRJ_P}`,
    org: ORG_A,
    service: "inventaris",
    project: PRJ_P,
    access: fakeJwt(claims),
    claims,
    issuedAt: now,
    expiresAt: now + 1_800_000,
    refreshAt: now + 600_000,
  };
}

const contexts = {
  currentOrganizationId: ORG_A,
  organizations: [
    { id: ORG_A, name: "Nusapala", slug: "nusapala", projects: [{ id: PRJ_P, name: "Parkir", slug: "parkir", services: ["inventaris"] }], projectsTruncated: false },
  ],
};

const signedIn: SessionState = {
  phase: "signed-in",
  identity: { userinfo: { sub: SUBJECT, name: "Siti Rahma", email: "siti@nusapala.id" }, orgId: ORG_A },
  tokens: [],
  active: { org: ORG_A },
  contexts,
};

const noDiscovery: SessionState = { ...signedIn, contexts: undefined, contextsError: "Konteks tidak dapat dibaca: invalid_scope" };

const item: Item = { id: "itm-1", name: "Kabel UTP Cat6", quantity: 12, createdBy: "usr_01a17b49-56bc-7632-a32f-d47311c068c1", createTime: "2026-11-01T10:00:00Z", updateTime: "2026-11-02T10:00:00Z" };
const withItems: SessionState = { ...signedIn, active: { org: ORG_A, project: PRJ_P }, tokens: [inventarisToken(FULL)], items: [item] };

const render = (state: SessionState) => renderToStaticMarkup(<Home session={session} state={state} />);
const disabled = (markup: string, action: string) => new RegExp(`<button[^>]*data-action="${action}"[^>]*disabled=""|<button[^>]*disabled=""[^>]*data-action="${action}"`).test(markup);

describe("Home", () => {
  it("shows the identity from userinfo and the organization from the client token", () => {
    const markup = render(signedIn);

    expect(markup).toMatch(/<dt>Email<\/dt><dd>siti@nusapala.id<\/dd>/);
    expect(markup).toContain(ORG_A);
  });

  it("shows the active context as a breadcrumb with names from the discovery", () => {
    const markup = render({ ...withItems, contexts });

    expect(markup).toMatch(/Organisasi<\/span> <strong>Nusapala<\/strong>/);
    expect(markup).toMatch(/Project<\/span> <strong>Parkir<\/strong>/);
  });

  it("falls back to short ids in the breadcrumb for a manual context", () => {
    const markup = render({ ...withItems, contexts: undefined, contextsError: "x" });

    expect(markup).toMatch(/Project<\/span> <strong>prj_0000…0001<\/strong>/);
  });

  it("shows the chooser from /me/contexts and asks to choose a project before listing items", () => {
    const markup = render(signedIn);

    expect(markup).toContain(`data-context="${ORG_A}/${PRJ_P}"`);
    expect(markup).toMatch(/Pilih project/);
    expect(markup).not.toMatch(/Gunakan konteks ini/);
  });

  it("shows a refused switch and keeps the chooser", () => {
    const markup = render({ ...signedIn, active: { org: ORG_A, project: PRJ_P }, switchError: "NICE menolak: invalid_scope" });

    expect(markup).toContain("NICE menolak: invalid_scope");
    expect(markup).toContain(`data-context="${ORG_A}/${PRJ_P}"`);
  });
});

describe("Home without context discovery", () => {
  it("explains it in one neutral sentence and offers the manual form prefilled with nice_org_id", () => {
    const markup = render(noDiscovery);

    expect(markup).toMatch(/NICE tidak lagi menyediakan penemuan organisasi dan project untuk SPA publik/);
    expect(markup).not.toMatch(/callout error/);
    expect(markup).toMatch(/Pilih konteks manual/);
    expect(markup).toContain(`value="${ORG_A}"`);
    expect(markup).toMatch(/placeholder="prj_…"/);
    expect(markup).toMatch(/Gunakan konteks ini/);
    expect(markup).toContain("Konteks tidak dapat dibaca: invalid_scope");
  });

  it("lists the recent contexts of the tab", () => {
    const markup = render({ ...noDiscovery, recent: [{ org: ORG_A, project: PRJ_P }] });

    expect(markup).toMatch(/Terakhir dipakai/);
    expect(markup).toContain(`data-recent="${ORG_A}/${PRJ_P}"`);
  });

  it("omits the recent list when there is none", () => {
    expect(render({ ...noDiscovery, recent: [] })).not.toMatch(/Terakhir dipakai/);
  });

  it("shows a refused manual context kindly and keeps the form", () => {
    const markup = render({ ...noDiscovery, switchError: "Konteks itu tidak dapat dipakai, Anda tetap di konteks sebelumnya. NICE menolak: ... (access_denied)." });

    expect(markup).toMatch(/callout warning/);
    expect(markup).toMatch(/access_denied/);
    expect(markup).toMatch(/Gunakan konteks ini/);
  });

  it("keeps showing the form while the contexts load again", () => {
    expect(render({ ...noDiscovery, contextsLoading: true })).toMatch(/Gunakan konteks ini/);
  });

  it("only says the contexts are loading while no answer or error exists", () => {
    expect(render(signedIn)).not.toMatch(/Memuat konteks/);
    expect(render({ ...signedIn, contexts: undefined })).toMatch(/Memuat konteks…/);
  });
});

describe("items", () => {
  it("asks to choose a context first, with a link to the panel", () => {
    const markup = render(signedIn);

    expect(markup).toMatch(/Belum ada konteks/);
    expect(markup).toContain('href="#konteks"');
    expect(markup).not.toMatch(/data-action="add"/);
  });

  it("lists the items with a short creator, a relative time and one row per item", () => {
    const markup = render(withItems);

    expect(markup).toContain("Kabel UTP Cat6");
    expect(markup).toContain("usr_01a1…68c1");
    expect(markup).toMatch(/<time dateTime="2026-11-02T10:00:00Z"/);
    expect(markup).toMatch(/1 barang/);
    expect(markup.match(/class="item-row"/g)).toHaveLength(1);
  });

  it("enables every action when the token carries all four permissions", () => {
    const markup = render(withItems);

    expect(disabled(markup, "add")).toBe(false);
    expect(disabled(markup, "edit")).toBe(false);
    expect(disabled(markup, "delete")).toBe(false);
    expect(markup).not.toMatch(/dinonaktifkan/);
  });

  it("disables what perms does not allow and says which permission is missing", () => {
    const markup = render({ ...withItems, tokens: [inventarisToken(["inventaris.items.read", "inventaris.items.edit"])] });

    expect(disabled(markup, "add")).toBe(true);
    expect(disabled(markup, "edit")).toBe(false);
    expect(disabled(markup, "delete")).toBe(true);
    expect(markup).toMatch(/Tambah, Hapus dinonaktifkan: token project ini tidak memuat inventaris.items.create, inventaris.items.delete/);
  });

  it("holds the buttons back when the permissions of the token are unknown", () => {
    const markup = render({ ...withItems, tokens: [] });

    expect(disabled(markup, "add")).toBe(true);
    expect(markup).toMatch(/izin di token belum diketahui/);
  });

  it("has its own loading, error and empty views", () => {
    expect(render({ ...withItems, items: undefined })).toMatch(/Memuat barang dari inventaris/);
    const failed = render({ ...withItems, items: undefined, itemsError: "inventaris menjawab HTTP 503." });
    expect(failed).toMatch(/Barang tidak dapat dimuat/);
    expect(failed).toContain("inventaris menjawab HTTP 503.");
    expect(failed).toMatch(/Coba lagi/);
    const empty = render({ ...withItems, items: [] });
    expect(empty).toMatch(/Belum ada barang di project ini/);
    expect(empty).toMatch(/Tambah barang/);
  });

  it("does not offer to add the first item without the permission", () => {
    const markup = render({ ...withItems, items: [], tokens: [inventarisToken(["inventaris.items.read"])] });

    expect(markup).toMatch(/Token Anda tidak boleh menambah barang/);
  });
});

describe("token status", () => {
  it("shows the badge, the claims as chips and never a refresh token", () => {
    const markup = render(withItems);

    expect(markup).toMatch(/>Aktif</);
    expect(markup).toMatch(/Lihat klaim lengkap/);
    expect(markup).toMatch(/<code class="chip">inventaris<\/code>/);
    expect(markup).toContain("inventaris.items.delete");
    expect(markup).not.toMatch(/refresh_token/);
  });

  it("marks a token past its refresh time as soon to be renewed and one past exp as expired", () => {
    const now = Date.now();
    const soon = { ...inventarisToken(FULL, now), refreshAt: now - 1000 };
    const expired = { ...inventarisToken(FULL, now), expiresAt: now - 1000 };

    expect(render({ ...withItems, tokens: [soon] })).toMatch(/Segera diperbarui/);
    expect(render({ ...withItems, tokens: [expired] })).toMatch(/Kedaluwarsa/);
  });
});
