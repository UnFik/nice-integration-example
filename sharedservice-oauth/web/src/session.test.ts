import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Session } from "./session";
import { FakeBackend, json, ORG_A, ORG_B, ORG_C, PRJ_P, PRJ_Q, PRJ_R } from "./test/fake-backend";
import { MemoryStorage } from "./test/memory-storage";

let backend: FakeBackend;
let navigations: string[];
let replaced: string[];

beforeEach(() => {
  backend = new FakeBackend();
  navigations = [];
  replaced = [];
});

function session(): Session {
  return new Session({
    fetch: backend.fetch,
    navigate: (url) => navigations.push(url),
    replaceUrl: (path) => replaced.push(path),
  });
}

async function started(href = "http://localhost:5181/"): Promise<Session> {
  const current = session();
  await current.start(href);
  return current;
}

describe("sign in", () => {
  it("shows Masuk without a session and navigates to /auth/login", async () => {
    backend.signedIn = false;

    const current = await started();
    expect(current.getState()).toEqual({ phase: "signed-out", perms: [] });

    current.signIn();
    current.signIn({ selectAccount: true });
    expect(navigations).toEqual(["/auth/login", "/auth/login?prompt=select_account"]);
  });

  it("loads the session, the choices and the items of the active context", async () => {
    const state = (await started()).getState();

    expect(state).toMatchObject({
      phase: "signed-in",
      user: { id: "usr_1", name: "Siti Rahma", email: "siti@nusapala.id" },
      active: { org: ORG_A, project: PRJ_P },
    });
    expect(state.perms).toContain("inventaris.items.delete");
    expect(state.contexts?.organizations.map((org) => org.id)).toEqual([ORG_A, ORG_B, ORG_C]);
    expect(state.items?.map((entry) => entry.name)).toEqual(["Kabel UTP Cat6"]);
  });

  it("explains access_denied from the callback, clears it from the URL, and offers another account", async () => {
    const current = await started("http://localhost:5181/?login_error=access_denied#x");

    expect(replaced).toEqual(["/#x"]);
    expect(current.getState()).toMatchObject({ phase: "login-failed", loginError: "access_denied" });
    expect(backend.requests).toEqual([]);
    current.signIn({ selectAccount: true });
    expect(navigations).toEqual(["/auth/login?prompt=select_account"]);
  });

  it("keeps other query parameters when it clears login_error", async () => {
    await started("http://localhost:5181/?tab=1&login_error=state_mismatch");

    expect(replaced).toEqual(["/?tab=1"]);
  });

  it("waits for a choice when the session has no context yet", async () => {
    backend.context = null;

    const state = (await started()).getState();

    expect(state.phase).toBe("signed-in");
    expect(state.active).toBeUndefined();
    expect(state.items).toBeUndefined();
    expect(backend.calls("GET", "/items")).toEqual([]);
    expect(state.contexts).toBeDefined();
  });

  it("says the backend is unreachable and retries", async () => {
    backend.next["/auth/session"] = [json(502, { error: "bad_gateway" })];

    const current = await started();
    expect(current.getState().phase).toBe("unreachable");

    await current.retry();
    expect(current.getState().phase).toBe("signed-in");
  });
});

describe("only the backend", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("calls relative URLs with same-origin credentials and the CSRF token on every write", async () => {
    const current = await started();
    await current.saveItem({ name: "Obeng", quantity: 3 });
    await current.choose(ORG_B, PRJ_R);
    await current.logout();

    for (const request of backend.requests) {
      expect(request.url.startsWith("/")).toBe(true);
      expect(request.credentials).toBe("same-origin");
      expect(request.headers.has("Authorization")).toBe(false);
      if (request.method === "GET") expect(request.headers.has("X-CSRF-Token")).toBe(false);
      else expect(request.headers.get("X-CSRF-Token")).toBe("csrf-1");
    }
  });

  it("leaves localStorage and sessionStorage empty through a whole session", async () => {
    const local = new MemoryStorage();
    const tab = new MemoryStorage();
    vi.stubGlobal("localStorage", local);
    vi.stubGlobal("sessionStorage", tab);

    const current = await started();
    await current.choose(ORG_B, PRJ_R);
    await current.saveItem({ name: "Obeng", quantity: 3 });
    await current.logout();

    expect(local.length).toBe(0);
    expect(tab.length).toBe(0);
  });

});

describe("switching context", () => {
  it("moves to another organization without leaving the page and lists its items", async () => {
    const current = await started();

    await expect(current.choose(ORG_B, PRJ_R)).resolves.toBe(true);

    expect(backend.calls("POST", "/auth/context")[0].body).toEqual({ org: ORG_B, project: PRJ_R });
    expect(current.getState()).toMatchObject({ active: { org: ORG_B, project: PRJ_R }, switching: undefined, switchError: undefined });
    expect(current.getState().items?.map((entry) => entry.name)).toEqual(["Router"]);
    expect(navigations).toEqual([]);
  });

  it("takes the permissions of the new context", async () => {
    const current = await started();

    await current.choose(ORG_A, PRJ_Q);

    expect(current.getState().perms).toEqual(["inventaris.items.read"]);
  });

  it("keeps the context on 409, shows the reason and reloads the choices", async () => {
    const current = await started();
    backend.refused[`${ORG_B}/${PRJ_R}`] = "invalid_scope";
    const before = backend.calls("GET", "/auth/contexts").length;

    await expect(current.choose(ORG_B, PRJ_R)).resolves.toBe(false);

    const state = current.getState();
    expect(state.active).toEqual({ org: ORG_A, project: PRJ_P });
    expect(state.items?.map((entry) => entry.name)).toEqual(["Kabel UTP Cat6"]);
    expect(state.switching).toBeUndefined();
    expect(state.switchError).toMatch(/tetap di konteks sebelumnya/);
    expect(state.switchError).toMatch(/invalid_scope/);
    expect(backend.calls("GET", "/auth/contexts").length).toBe(before + 1);
  });

  it("explains access_denied on a refused switch", async () => {
    const current = await started();
    backend.refused[`${ORG_B}/${PRJ_R}`] = "access_denied";

    await current.choose(ORG_B, PRJ_R);

    expect(current.getState().switchError).toMatch(/tidak boleh memakai inventaris di organisasi itu \(access_denied\)/);
  });

  it("renews the CSRF token once after 403 csrf and sends the write again", async () => {
    const current = await started();
    backend.csrf = "csrf-2";

    await expect(current.choose(ORG_B, PRJ_R)).resolves.toBe(true);

    expect(backend.calls("POST", "/auth/context").map((request) => request.headers.get("X-CSRF-Token"))).toEqual(["csrf-1", "csrf-2"]);
  });
});

describe("revoked access", () => {
  it("shows the reason from the session and the picker, and lists nothing", async () => {
    backend.contextError = "access_denied";

    const state = (await started()).getState();

    expect(state).toMatchObject({ phase: "signed-in", contextError: "access_denied", perms: [] });
    expect(backend.calls("GET", "/items")).toEqual([]);
    expect(state.contexts).toBeDefined();
  });

  it("turns a 409 context_refused of the items into the reason", async () => {
    const current = await started();
    backend.contextError = "invalid_scope";

    await current.loadItems();

    expect(current.getState()).toMatchObject({ contextError: "invalid_scope", perms: [], items: undefined });
  });

  it("explains a delete refused because access was revoked at NICE", async () => {
    const current = await started();
    backend.next["/items/itm_1"] = [json(403, { error: "access_denied" })];

    const result = await current.removeItem("itm_1");

    expect(result).toMatchObject({ ok: false, message: expect.stringMatching(/dicabut/) });
  });
});

describe("discovery failure", () => {
  it("shows the cause and keeps the active context and its items", async () => {
    backend.contextsFailure = json(502, { error: "contexts_unavailable", error_description: "invalid_grant" });

    const state = (await started()).getState();

    expect(state.contextsError).toMatch(/NICE menolak penemuan konteks \(invalid_grant\)/);
    expect(state.contextsError).toMatch(/Konteks aktif tetap dipakai/);
    expect(state.active).toEqual({ org: ORG_A, project: PRJ_P });
    expect(state.items).toHaveLength(1);
  });

  it("names a NICE that did not answer", async () => {
    backend.contextsFailure = json(503, { error: "temporarily_unavailable" });

    expect((await started()).getState().contextsError).toMatch(/NICE tidak menjawab/);
  });
});

describe("session ended", () => {
  it("shows Sesi berakhir when NICE ended the session during a call", async () => {
    const current = await started();
    backend.ended = true;

    const result = await current.saveItem({ name: "Obeng", quantity: 1 });

    expect(result.ok).toBe(false);
    expect(current.getState()).toEqual({ phase: "ended", perms: [] });
  });

  it("shows Sesi berakhir when the page loads after NICE ended it", async () => {
    backend.ended = true;

    expect((await started()).getState().phase).toBe("ended");
  });

  it("drops a session the backend no longer knows to the signed-out screen", async () => {
    const current = await started();
    backend.signedIn = false;

    await current.choose(ORG_B, PRJ_R);

    expect(current.getState().phase).toBe("signed-out");
  });
});

describe("items", () => {
  it("adds, edits and deletes, then reloads the list", async () => {
    const current = await started();

    expect(await current.saveItem({ name: "Obeng", quantity: 3 })).toEqual({ ok: true });
    expect(await current.saveItem({ name: "Kabel", quantity: 13 }, "itm_1")).toEqual({ ok: true });
    expect(await current.removeItem("itm_1")).toEqual({ ok: true });

    expect(current.getState().items?.map((entry) => [entry.name, entry.quantity])).toEqual([["Obeng", 3]]);
  });

  it("does not list items without the read permission", async () => {
    backend.perms[`${ORG_A}/${PRJ_P}`] = ["inventaris.items.create"];

    const state = (await started()).getState();

    expect(backend.calls("GET", "/items")).toEqual([]);
    expect(state.itemsError).toMatch(/inventaris.items.read/);
  });

  it("answers a refused write with one sentence and keeps the list", async () => {
    const current = await started();
    await current.choose(ORG_A, PRJ_Q);

    const result = await current.saveItem({ name: "Obeng", quantity: 1 });

    expect(result).toMatchObject({ ok: false, message: expect.stringMatching(/tidak punya izin untuk menambah barang/) });
  });
});

describe("sign out", () => {
  it("removes the session and shows the signed-out screen", async () => {
    const current = await started();

    await current.logout();

    expect(current.getState()).toMatchObject({ phase: "signed-out", signedOut: true });
    expect(navigations).toEqual([]);
  });

  it("keeps the session when NICE did not answer, then succeeds on retry", async () => {
    const current = await started();
    backend.logoutFailure = json(503, { error: "temporarily_unavailable" });

    await current.logout();

    const state = current.getState();
    expect(state.phase).toBe("signed-in");
    expect(state.loggingOut).toBe(false);
    expect(state.logoutError).toMatch(/belum keluar/);
    expect(state.items).toHaveLength(1);

    backend.logoutFailure = undefined;
    await current.logout();
    expect(current.getState()).toMatchObject({ phase: "signed-out", signedOut: true });
  });
});
