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
