import { beforeEach, describe, expect, it } from "vitest";
import type { AppConfig } from "./config";
import { BASE_SCOPE } from "./scope";
import { Session } from "./session";
import { keys } from "./storage";
import {
  CLIENT_ID,
  EMAIL,
  FakeNice,
  fakeInventaris,
  json,
  INVENTARIS_URL,
  ORG_A,
  ORG_B,
  ORG_C,
  PRJ_P,
  PRJ_R,
  POST_LOGOUT_URI,
  REDIRECT_URI,
  SUBJECT,
} from "./test/fake-nice";
import { MemoryStorage } from "./test/memory-storage";

let nice: FakeNice;
let storage: MemoryStorage;
let navigations: string[];
let replaced: string[];

beforeEach(() => {
  nice = new FakeNice();
  nice.service = fakeInventaris();
  storage = new MemoryStorage();
  navigations = [];
  replaced = [];
});

function session(): Session {
  const config: AppConfig = {
    issuer: nice.issuer,
    clientId: CLIENT_ID,
    redirectUri: REDIRECT_URI,
    postLogoutRedirectUri: POST_LOGOUT_URI,
    inventarisUrl: INVENTARIS_URL,
  };
  return new Session(config, {
    storage,
    fetch: nice.fetch,
    navigate: (url) => navigations.push(url),
    replaceUrl: (path) => replaced.push(path),
  });
}

async function signedIn(): Promise<Session> {
  const first = session();
  await first.resume();
  await first.signIn();
  const callback = nice.authorize(navigations[0]);
  navigations = [];
  const current = session();
  await current.completeLogin(callback);
  return current;
}

const storedKeys = () => [...Array(storage.length).keys()].map((i) => storage.key(i)).sort();
const refreshes = () => nice.tokenRequests.filter((form) => form.get("grant_type") === "refresh_token");
const claimsOf = (authorization: string | null) =>
  JSON.parse(atob((authorization ?? "").slice(7).split(".")[1].replace(/-/g, "+").replace(/_/g, "/")));

describe("login", () => {
  it("lands in the first organization entitled to inventaris and shows it as current", async () => {
    nice.org(ORG_A)!.entitled = false;

    const current = await signedIn();
    const state = current.getState();

    expect(replaced).toEqual(["/"]);
    expect(state.phase).toBe("signed-in");
    expect(state.active).toEqual({ org: ORG_B });
    expect(state.identity).toMatchObject({ userinfo: { sub: SUBJECT, name: "Siti Rahma", email: EMAIL }, orgId: ORG_B });
    expect(state.contexts?.organizations.map((org) => org.id)).toEqual([ORG_B, ORG_C]);
  });

  it("asks userinfo with the client token, then /me/contexts with the delegated token of that organization", async () => {
    const current = await signedIn();
    const client = current.getState().tokens.find((held) => !held.service);

    expect(nice.userinfoRequests).toEqual([{ authorization: `Bearer ${client?.access}` }]);
    expect(refreshes().map((form) => form.get("scope"))).toEqual([`${BASE_SCOPE} nice:aud:nice`]);
    expect(claimsOf(nice.contextsRequests[0].authorization)).toMatchObject({ aud: "nice", nice_org_id: ORG_A });
    expect(new URL(nice.contextsRequests[0].url).searchParams.has("service")).toBe(false);
  });

  it("keeps only the refresh token and the ID token in sessionStorage", async () => {
    const current = await signedIn();

    expect(storedKeys()).toEqual([keys.idToken, keys.refreshToken].sort());
    const stored = storedKeys().map((key) => storage.getItem(key ?? ""));
    for (const held of current.getState().tokens) expect(stored).not.toContain(held.access);
  });

  it("explains access_denied and offers to sign in with another account", async () => {
    for (const org of nice.orgs) org.entitled = false;
    const first = session();
    await first.resume();
    await first.signIn();
    const callback = nice.authorize(navigations[0]);
    navigations = [];

    const current = session();
    await current.completeLogin(callback);

    expect(current.getState()).toMatchObject({ phase: "login-failed", reason: "service_unavailable" });
    await current.signIn({ selectAccount: true });
    expect(new URL(navigations[0]).searchParams.get("prompt")).toBe("select_account");
  });
});

describe("reload", () => {
  it("stays signed in, in the organization of the refresh token, without a redirect", async () => {
    await signedIn();
    nice.sessionOrg = ORG_B;

    const reloaded = session();
    await reloaded.resume();

    const state = reloaded.getState();
    expect(state.phase).toBe("signed-in");
    expect(navigations).toEqual([]);
    expect(state.active).toEqual({ org: ORG_B });
    expect(state.identity?.userinfo.sub).toBe(SUBJECT);
    expect(state.contexts?.currentOrganizationId).toBe(ORG_B);
  });

  it("is signed out in a tab without the session", async () => {
    const fresh = session();
    await fresh.resume();

    expect(fresh.getState().phase).toBe("signed-out");
  });

  it("ends the session when NICE answers invalid_grant", async () => {
    await signedIn();
    nice.refreshToken = "revoked";

    const reloaded = session();
    await reloaded.resume();

    expect(reloaded.getState()).toMatchObject({ phase: "ended", tokens: [] });
    expect(storage.length).toBe(0);
  });
});

describe("contexts", () => {
  it("shows why the contexts could not be read without ending the session", async () => {
    const current = await signedIn();
    nice.contextsOverride = () => json(503, { code: 503, error_code: "unavailable", message: "try again" });

    await current.reloadContexts();

    expect(current.getState().phase).toBe("signed-in");
    expect(current.getState().contextsError).toMatch(/Konteks tidak dapat dibaca/);
  });
});

describe("switching context", () => {
  it("moves from A/P to B/R with one refresh, without leaving the page, and lists the items of B/R", async () => {
    const current = await signedIn();
    await current.choose(ORG_A, PRJ_P);
    const sent = refreshes().length;

    await current.choose(ORG_B, PRJ_R);

    const state = current.getState();
    expect(refreshes().slice(sent).map((form) => form.get("scope"))).toEqual([
      `${BASE_SCOPE} nice:org:${ORG_B} nice:aud:inventaris nice:prj:${PRJ_R}`,
    ]);
    expect(navigations).toEqual([]);
    expect(state.active).toEqual({ org: ORG_B, project: PRJ_R });
    expect(state.switchError).toBeUndefined();
    expect(state.items?.map((item) => item.id)).toEqual([`itm-${ORG_B}-${PRJ_R}`]);
    expect(claimsOf(nice.serviceRequests.at(-1)?.authorization ?? null)).toMatchObject({ aud: "inventaris", nice_org_id: ORG_B });
  });

  it("returns to A/P within ten minutes with the token it holds, without asking NICE", async () => {
    const current = await signedIn();
    await current.choose(ORG_A, PRJ_P);
    await current.choose(ORG_B, PRJ_R);
    const sent = nice.tokenRequests.length;

    await current.choose(ORG_A, PRJ_P);

    expect(nice.tokenRequests).toHaveLength(sent);
    expect(current.getState().active).toEqual({ org: ORG_A, project: PRJ_P });
    expect(current.getState().items?.map((item) => item.id)).toEqual([`itm-${ORG_A}-${PRJ_P}`]);
  });

  it("stays on the previous context, shows the reason, and reloads the contexts on invalid_scope", async () => {
    const current = await signedIn();
    await current.choose(ORG_A, PRJ_P);
    const before = storage.getItem(keys.refreshToken);
    const read = nice.contextsRequests.length;
    nice.org(ORG_B)!.projects[0].services = [];

    await current.choose(ORG_B, PRJ_R);

    const state = current.getState();
    expect(state.phase).toBe("signed-in");
    expect(state.active).toEqual({ org: ORG_A, project: PRJ_P });
    expect(state.items?.map((item) => item.id)).toEqual([`itm-${ORG_A}-${PRJ_P}`]);
    expect(state.switchError).toMatch(/invalid_scope/);
    expect(storage.getItem(keys.refreshToken)).toBe(before);
    expect(nice.contextsRequests.length).toBe(read + 1);
    expect(state.contexts?.organizations.find((org) => org.id === ORG_B)?.projects).toEqual([]);
  });

  it("stays on the previous context and reloads the contexts on access_denied", async () => {
    const current = await signedIn();
    await current.choose(ORG_A, PRJ_P);
    nice.org(ORG_B)!.entitled = false;

    await current.choose(ORG_B, PRJ_R);

    const state = current.getState();
    expect(state.active).toEqual({ org: ORG_A, project: PRJ_P });
    expect(state.switchError).toMatch(/access_denied/);
    expect(state.contexts?.organizations.map((org) => org.id)).toEqual([ORG_A, ORG_C]);
  });

  it("ends the session when NICE answers invalid_grant during a switch", async () => {
    const current = await signedIn();
    nice.refreshToken = "revoked";

    await current.choose(ORG_B, PRJ_R);

    expect(current.getState()).toMatchObject({ phase: "ended", tokens: [] });
    expect(storage.length).toBe(0);
  });

  it("shows a call to inventaris that fails, in the chosen context", async () => {
    const current = await signedIn();
    nice.service = () => json(403, { error: "permission_denied" });

    await current.choose(ORG_A, PRJ_P);

    expect(current.getState().active).toEqual({ org: ORG_A, project: PRJ_P });
    expect(current.getState().itemsError).toMatch(/403/);
  });
});

// NICE no longer answers the delegated nice:aud:nice refresh for service clients.
const noDiscovery = (form: URLSearchParams) =>
  form.get("grant_type") === "refresh_token" && form.get("scope")?.includes("nice:aud:nice")
    ? json(400, { error: "invalid_scope", error_description: "nice:aud:nice is not allowed" })
    : undefined;

describe("without context discovery", () => {
  it("stays signed in with the contexts error and no picker data", async () => {
    nice.tokenOverride = noDiscovery;

    const current = await signedIn();
    const state = current.getState();

    expect(state.phase).toBe("signed-in");
    expect(state.contexts).toBeUndefined();
    expect(state.contextsError).toMatch(/Konteks tidak dapat dibaca/);
    expect(state.active).toEqual({ org: ORG_A });
    expect(state.recent).toEqual([]);
  });

  it("moves to a typed context, loads its items and remembers it as ids only", async () => {
    nice.tokenOverride = noDiscovery;
    const current = await signedIn();

    const accepted = await current.chooseManual(ORG_B, PRJ_R);

    const state = current.getState();
    expect(accepted).toBe(true);
    expect(state.active).toEqual({ org: ORG_B, project: PRJ_R });
    expect(state.items?.map((entry) => entry.id)).toEqual([`itm-${ORG_B}-${PRJ_R}`]);
    expect(state.recent).toEqual([{ org: ORG_B, project: PRJ_R }]);
    const stored = storage.getItem("nice-service-ui.recent_contexts") ?? "";
    expect(JSON.parse(stored)).toEqual([{ org: ORG_B, project: PRJ_R }]);
    expect(stored).not.toMatch(/\./);
    expect(refreshes().at(-1)?.get("scope")).toBe(`${BASE_SCOPE} nice:org:${ORG_B} nice:aud:inventaris nice:prj:${PRJ_R}`);
  });

  it("shows access_denied kindly, keeps the context and remembers nothing", async () => {
    nice.tokenOverride = noDiscovery;
    const current = await signedIn();
    await current.chooseManual(ORG_A, PRJ_P);
    nice.org(ORG_B)!.entitled = false;
    const before = storage.getItem(keys.refreshToken);
    const asked = nice.contextsRequests.length;

    const accepted = await current.chooseManual(ORG_B, PRJ_R);

    const state = current.getState();
    expect(accepted).toBe(false);
    expect(state.switchError).toMatch(/NICE menolak: akun Anda tidak boleh memakai inventaris/);
    expect(state.switchError).toMatch(/access_denied/);
    expect(state.active).toEqual({ org: ORG_A, project: PRJ_P });
    expect(state.recent).toEqual([{ org: ORG_A, project: PRJ_P }]);
    expect(storage.getItem(keys.refreshToken)).toBe(before);
    expect(nice.contextsRequests.length).toBe(asked);
  });

  it("shows invalid_scope kindly for a project that does not use inventaris", async () => {
    nice.tokenOverride = noDiscovery;
    const current = await signedIn();
    nice.org(ORG_B)!.projects[0].services = [];

    await current.chooseManual(ORG_B, PRJ_R);

    expect(current.getState().switchError).toMatch(/project itu tidak ada, tidak mengaktifkan inventaris/);
    expect(current.getState().switchError).toMatch(/invalid_scope/);
    expect(current.getState().active).toEqual({ org: ORG_A });
  });

  it("forgets the recent contexts with the session", async () => {
    nice.tokenOverride = noDiscovery;
    const current = await signedIn();
    await current.chooseManual(ORG_A, PRJ_P);

    await current.logout();

    expect(storage.length).toBe(0);
  });

  it("keeps the recent contexts of the tab after a reload", async () => {
    nice.tokenOverride = noDiscovery;
    const first = await signedIn();
    await first.chooseManual(ORG_A, PRJ_P);

    const reloaded = session();

    expect(reloaded.getState().recent).toEqual([{ org: ORG_A, project: PRJ_P }]);
  });
});

describe("managing items", () => {
  const stored: { id: string; name: string; quantity: number }[] = [];

  function inventaris(): NonNullable<FakeNice["service"]> {
    return (url: URL, init: RequestInit) => {
      if (url.origin !== INVENTARIS_URL) return undefined;
      const method = init.method ?? "GET";
      if (method === "GET") return json(200, { items: stored });
      if (method === "POST") {
        const input = JSON.parse(String(init.body)) as { name: string; quantity: number };
        if (!input.name) return json(400, { error: "invalid_argument", error_description: "name" });
        stored.push({ id: `itm_${stored.length + 1}`, ...input });
        return json(201, stored.at(-1));
      }
      const id = url.pathname.split("/").pop();
      const index = stored.findIndex((entry) => entry.id === id);
      if (index < 0) return json(404, { error: "not_found" });
      if (method === "DELETE") {
        stored.splice(index, 1);
        return new Response(null, { status: 204 });
      }
      stored[index] = { ...stored[index], ...JSON.parse(String(init.body)) };
      return json(200, stored[index]);
    };
  }

  beforeEach(() => {
    stored.length = 0;
    nice.service = inventaris();
  });

  it("creates, edits and deletes in the active context and reloads the list each time", async () => {
    const current = await signedIn();
    await current.choose(ORG_A, PRJ_P);

    expect(await current.saveItem({ name: "Kabel", quantity: 3 })).toEqual({ ok: true });
    expect(current.getState().items?.map((entry) => entry.name)).toEqual(["Kabel"]);

    expect(await current.saveItem({ name: "Kabel UTP", quantity: 5 }, "itm_1")).toEqual({ ok: true });
    expect(current.getState().items?.[0]).toMatchObject({ name: "Kabel UTP", quantity: 5 });

    expect(await current.removeItem("itm_1")).toEqual({ ok: true });
    expect(current.getState().items).toEqual([]);
    expect(claimsOf(nice.serviceRequests.at(-1)?.authorization ?? null)).toMatchObject({ aud: "inventaris", prj: PRJ_P });
  });

  it("answers a refused change in a sentence and leaves the list as it was", async () => {
    const current = await signedIn();
    await current.choose(ORG_A, PRJ_P);
    stored.push({ id: "itm_1", name: "Kabel", quantity: 1 });
    await current.loadItems();

    const result = await current.saveItem({ name: "", quantity: 1 });

    expect(result).toMatchObject({ ok: false, message: expect.stringContaining("(name)") });
    expect(current.getState().items?.map((entry) => entry.id)).toEqual(["itm_1"]);
  });

  it("reloads the list when the item is already gone", async () => {
    const current = await signedIn();
    await current.choose(ORG_A, PRJ_P);
    stored.push({ id: "itm_1", name: "Kabel", quantity: 1 });
    await current.loadItems();
    stored.length = 0;

    const result = await current.removeItem("itm_1");

    expect(result).toMatchObject({ ok: false, message: expect.stringMatching(/tidak ditemukan/) });
    expect(current.getState().items).toEqual([]);
  });

  it("says a revoked access on delete came from NICE's online check", async () => {
    const current = await signedIn();
    await current.choose(ORG_A, PRJ_P);
    nice.service = () => json(403, { error: "access_denied" });

    const result = await current.removeItem("itm_1");

    expect(result).toMatchObject({ ok: false, message: expect.stringMatching(/dicabut.*diperiksa langsung ke NICE/) });
  });

  it("asks for a project before any change", async () => {
    const current = await signedIn();

    expect(await current.removeItem("itm_1")).toMatchObject({ ok: false, message: expect.stringMatching(/Pilih project/) });
  });
});
