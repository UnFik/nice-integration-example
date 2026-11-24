import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { Contexts } from "../api";
import type { Device } from "../devices";
import { Session, type SessionState } from "../session";
import { FULL, ORG_A, ORG_B, PRJ_P } from "../test/fake-backend";
import { Home } from "./Home";

const session = new Session({ fetch: () => Promise.reject(new Error("offline")), navigate: () => {}, replaceUrl: () => {} });

const contexts: Contexts = {
  currentOrganizationId: ORG_A,
  organizations: [
    { id: ORG_A, name: "Nusapala", slug: "nusapala", actAs: false, projects: [{ id: PRJ_P, name: "Parkir", slug: "parkir" }], projectsTruncated: false },
    { id: ORG_B, name: "Bintang Laut", slug: "bintang-laut", actAs: false, projects: [], projectsTruncated: false },
  ],
};

const device: Device = { id: "dev-1", name: "Sensor Suhu Gudang", serialNumber: "SN-2026-0001", status: "active", location: "Gudang A", createdBy: "usr_01a17b49-56bc-7632-a32f-d47311c068c1", createTime: "2026-11-01T10:00:00Z", updateTime: "2026-11-02T10:00:00Z" };

const signedIn: SessionState = {
  phase: "signed-in",
  user: { id: "usr_1", name: "Siti Rahma", email: "siti@nusapala.id" },
  active: { org: ORG_A, project: PRJ_P },
  perms: FULL,
  contexts,
  devices: [device],
};

const noContext: SessionState = { ...signedIn, active: undefined, perms: [], devices: undefined };

const render = (state: SessionState) => renderToStaticMarkup(<Home session={session} state={state} />);
const disabled = (markup: string, action: string) => new RegExp(`<button[^>]*data-action="${action}"[^>]*disabled=""|<button[^>]*disabled=""[^>]*data-action="${action}"`).test(markup);

describe("Home", () => {
  it("shows the identity and the permissions from /auth/session", () => {
    const markup = render(signedIn);

    expect(markup).toMatch(/<dt>Email<\/dt><dd>siti@nusapala.id<\/dd>/);
    expect(markup).toContain("<code>nadi.devices.delete</code>");
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
    const markup = render({ ...signedIn, contextError: "access_denied", perms: [], devices: undefined });

    expect(markup).toContain('data-context-error="access_denied"');
    expect(markup).toMatch(/tidak boleh memakai nadi di organisasi itu/);
    expect(markup).toMatch(/Konteks aktif tidak dapat dipakai/);
    expect(markup).not.toMatch(/Memuat perangkat/);
  });

  it("shows a failed discovery and keeps the devices", () => {
    const markup = render({ ...signedIn, contexts: undefined, contextsError: "Pilihan konteks tidak dapat dimuat: NICE menolak penemuan konteks (invalid_grant). Konteks aktif tetap dipakai." });

    expect(markup).toContain("NICE menolak penemuan konteks (invalid_grant)");
    expect(markup).toContain("Sensor Suhu Gudang");
    expect(markup).toContain("SN-2026-0001");
    expect(markup).toContain("Aktif");
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

describe("devices", () => {
  it("lists the devices with a short creator, a relative time and one row per device", () => {
    const markup = render(signedIn);

    expect(markup).toContain("Sensor Suhu Gudang");
    expect(markup).toContain("usr_01a1…68c1");
    expect(markup).toMatch(/<time dateTime="2026-11-02T10:00:00Z"/);
    expect(markup).toMatch(/1 perangkat/);
    expect(markup.match(/class="device-row"/g)).toHaveLength(1);
  });

  it("enables every action when perms carries all four permissions", () => {
    const markup = render(signedIn);

    expect(disabled(markup, "add")).toBe(false);
    expect(disabled(markup, "edit")).toBe(false);
    expect(disabled(markup, "delete")).toBe(false);
    expect(markup).not.toMatch(/dinonaktifkan/);
  });

  it("disables what perms does not allow and says which permission is missing", () => {
    const markup = render({ ...signedIn, perms: ["nadi.devices.read", "nadi.devices.edit"] });

    expect(disabled(markup, "add")).toBe(true);
    expect(disabled(markup, "edit")).toBe(false);
    expect(disabled(markup, "delete")).toBe(true);
    expect(markup).toMatch(/Tambah, Hapus dinonaktifkan: token konteks ini tidak memuat nadi.devices.create, nadi.devices.delete/);
  });

  it("has its own loading, error and empty views", () => {
    expect(render({ ...signedIn, devices: undefined })).toMatch(/Memuat perangkat…/);
    const failed = render({ ...signedIn, devices: undefined, devicesError: "nadi belum bisa memeriksa akses ke NICE." });
    expect(failed).toMatch(/Perangkat tidak dapat dimuat/);
    expect(failed).toMatch(/Coba lagi/);
    const empty = render({ ...signedIn, devices: [] });
    expect(empty).toMatch(/Belum ada perangkat di project ini/);
    expect(empty).toMatch(/Tambah perangkat/);
  });

  it("does not offer to add the first device without the permission", () => {
    expect(render({ ...signedIn, devices: [], perms: ["nadi.devices.read"] })).toMatch(/Token Anda tidak boleh menambah perangkat/);
  });
});
