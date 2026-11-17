import { beforeEach, describe, expect, it, vi } from "vitest";
import { OidcClient } from "./oidc";
import { BASE_SCOPE } from "./scope";
import { keys } from "./storage";
import {
  CLIENT_ID,
  FakeNice,
  fakeJwt,
  json,
  LOGIN_SCOPES,
  ORG_A,
  ORG_B,
  POST_LOGOUT_URI,
  PRJ_P,
  PRJ_Q,
  PRJ_R,
  REDIRECT_URI,
} from "./test/fake-nice";
import { MemoryStorage } from "./test/memory-storage";
import { type TokenKey, TokenError, TokenManager, type TokenManagerOptions } from "./token-manager";

const MINUTE = 60_000;
const ITEMS = "http://127.0.0.1:8081/items";
const client: TokenKey = {};
const at = (org: string, project: string): TokenKey => ({ org, service: "inventaris", project });
const niceIn = (org: string): TokenKey => ({ org, service: "nice" });
const AP = at(ORG_A, PRJ_P);
const AQ = at(ORG_A, PRJ_Q);
const BR = at(ORG_B, PRJ_R);

let now: number;
let nice: FakeNice;
let storage: MemoryStorage;
let ended: ReturnType<typeof vi.fn<(error: TokenError) => void>>;

beforeEach(() => {
  now = Date.parse("2026-11-13T03:00:00Z");
  nice = new FakeNice(() => now);
  storage = new MemoryStorage();
  storage.setItem(keys.refreshToken, nice.refreshToken);
  ended = vi.fn<(error: TokenError) => void>();
});

async function manager(options: Partial<TokenManagerOptions> = {}): Promise<TokenManager> {
  const oidc = await OidcClient.discover({
    issuer: nice.issuer,
    clientId: CLIENT_ID,
    redirectUri: REDIRECT_URI,
    postLogoutRedirectUri: POST_LOGOUT_URI,
    fetch: nice.fetch,
  });
  return new TokenManager({
    storage,
    grant: oidc.refreshGrant,
    fetch: nice.fetch,
    now: () => now,
    random: () => 0,
    onSessionEnded: ended,
    ...options,
  });
}

const refreshes = () => nice.tokenRequests.filter((form) => form.get("grant_type") === "refresh_token");
const scopes = () => refreshes().map((form) => form.get("scope"));
const claimsOf = (token: string) => JSON.parse(atob(token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")));

describe("organization of the session", () => {
  it("learns the organization from the first token, asked with the base scope only", async () => {
    const tokens = await manager();
    expect(tokens.currentOrg()).toBeUndefined();

    const access = await tokens.token(client);

    expect(scopes()).toEqual([BASE_SCOPE]);
    expect(claimsOf(access)).toMatchObject({ aud: CLIENT_ID, nice_org_id: ORG_A });
    expect(tokens.currentOrg()).toBe(ORG_A);
    expect(tokens.snapshot()).toEqual([expect.objectContaining({ key: `${ORG_A}||`, org: ORG_A })]);
  });

  it("learns the organization from a held token of the code exchange", async () => {
    const tokens = await manager();

    tokens.hold(client, nice.accessToken(CLIENT_ID, ORG_B), 1800);

    expect(tokens.currentOrg()).toBe(ORG_B);
    expect(refreshes()).toHaveLength(0);
  });

  it("asks for inventaris in a project of the same organization without nice:org", async () => {
    const tokens = await manager();
    await tokens.token(client);

    const access = await tokens.token(AP);

    expect(scopes().at(-1)).toBe(`${BASE_SCOPE} nice:aud:inventaris nice:prj:${PRJ_P}`);
    expect(claimsOf(access)).toMatchObject({ aud: "inventaris", prj: PRJ_P, nice_org_id: ORG_A });
  });

  it("switches organization with one refresh carrying nice:org, then stays there without it", async () => {
    const tokens = await manager();
    await tokens.token(AP);

    const access = await tokens.token(BR);
    await tokens.token(niceIn(ORG_B));

    expect(scopes().slice(-2)).toEqual([
      `${BASE_SCOPE} nice:org:${ORG_B} nice:aud:inventaris nice:prj:${PRJ_R}`,
      `${BASE_SCOPE} nice:aud:nice`,
    ]);
    expect(claimsOf(access)).toMatchObject({ nice_org_id: ORG_B, prj: PRJ_R });
    expect(tokens.currentOrg()).toBe(ORG_B);
  });

  it("reuses the token of the previous context without asking NICE, and moves back on the next refresh", async () => {
    const tokens = await manager();
    const first = await tokens.token(AP);
    await tokens.token(BR);
    now += 9 * MINUTE;
    const sent = refreshes().length;

    expect(await tokens.token(AP)).toBe(first);
    expect(refreshes()).toHaveLength(sent);
    expect(tokens.currentOrg()).toBe(ORG_B);

    await tokens.token(AQ);
    expect(scopes().at(-1)).toBe(`${BASE_SCOPE} nice:org:${ORG_A} nice:aud:inventaris nice:prj:${PRJ_Q}`);
    expect(tokens.currentOrg()).toBe(ORG_A);
  });

  it("decides nice:org when the refresh is sent, not when it is queued", async () => {
    nice.latencyMs = 20;
    const tokens = await manager();
    await tokens.token(client);

    const [b, a] = await Promise.all([tokens.token(BR), tokens.token(AQ)]);

    expect(claimsOf(b).nice_org_id).toBe(ORG_B);
    expect(claimsOf(a).nice_org_id).toBe(ORG_A);
    expect(scopes().slice(-2)).toEqual([
      `${BASE_SCOPE} nice:org:${ORG_B} nice:aud:inventaris nice:prj:${PRJ_R}`,
      `${BASE_SCOPE} nice:org:${ORG_A} nice:aud:inventaris nice:prj:${PRJ_Q}`,
    ]);
    expect(nice.maxInFlight).toBe(1);
  });

  it("keeps the session, its organization, and the refresh token when NICE refuses a switch with access_denied", async () => {
    const tokens = await manager();
    await tokens.token(AP);
    const before = storage.getItem(keys.refreshToken);
    nice.org(ORG_B)!.entitled = false;

    const refused = await tokens.token(BR).catch((error: unknown) => error);

    expect(refused).toBeInstanceOf(TokenError);
    expect(refused).toMatchObject({ kind: "refused", code: "access_denied" });
    expect(storage.getItem(keys.refreshToken)).toBe(before);
    expect(nice.refreshToken).toBe(before);
    expect(tokens.currentOrg()).toBe(ORG_A);
    expect(ended).not.toHaveBeenCalled();
    await expect(tokens.token(AQ)).resolves.toBeTruthy();
    expect(scopes().at(-1)).toBe(`${BASE_SCOPE} nice:aud:inventaris nice:prj:${PRJ_Q}`);
  });

  it("keeps the session when NICE refuses a project with invalid_scope", async () => {
    const tokens = await manager();
    await tokens.token(AP);
    nice.org(ORG_A)!.projects[1].services = [];

    await expect(tokens.token(AQ)).rejects.toMatchObject({ kind: "refused", code: "invalid_scope" });

    expect(ended).not.toHaveBeenCalled();
    expect(tokens.currentOrg()).toBe(ORG_A);
    await expect(tokens.token(niceIn(ORG_A))).resolves.toBeTruthy();
  });
});

describe("delegated token for NICE", () => {
  it("asks for nice:aud:nice with the base scope and without a project", async () => {
    const tokens = await manager();

    const access = await tokens.token(niceIn(ORG_A));

    expect(claimsOf(access)).toMatchObject({ aud: "nice", nice_service: "inventaris", perms: ["nice.me.contexts.read"] });
    expect(scopes()).toEqual([`${BASE_SCOPE} nice:org:${ORG_A} nice:aud:nice`]);
  });
});

describe("reuse and renewal", () => {
  it("reuses the token for two calls within a minute", async () => {
    const tokens = await manager();
    nice.service = () => json(200, { items: [] });

    await tokens.call(AP, ITEMS);
    now += 59_000;
    await tokens.call(AP, ITEMS);

    expect(refreshes()).toHaveLength(1);
    expect(nice.serviceRequests[0].authorization).toBe(nice.serviceRequests[1].authorization);
  });

  it("renews after iat + 10 minutes + jitter", async () => {
    const tokens = await manager({ random: () => 0.5 });
    const first = await tokens.token(AP);

    now += 10 * MINUTE + 29_000;
    expect(await tokens.token(AP)).toBe(first);
    now += 1_000;

    expect(await tokens.token(AP)).not.toBe(first);
    expect(refreshes()).toHaveLength(2);
  });

  it("renews a token that is within 60 seconds of exp", async () => {
    nice.accessTtlSeconds = 5 * 60;
    const tokens = await manager();
    const first = await tokens.token(AP);

    now += 3 * MINUTE + 59_000;
    expect(await tokens.token(AP)).toBe(first);
    now += 1_000;

    expect(await tokens.token(AP)).not.toBe(first);
  });

  it("keeps the client token on the base scope only, so userinfo still works after a service token", async () => {
    const tokens = await manager();
    await tokens.token(BR);

    const access = await tokens.token({ org: ORG_B });

    expect(scopes().at(-1)).toBe(BASE_SCOPE);
    expect(claimsOf(access).aud).toBe(CLIENT_ID);
    expect(nice.grantScopes).toEqual(LOGIN_SCOPES);
  });
});

describe("one refresh at a time", () => {
  it("sends refreshes one after the other, each with the latest refresh token", async () => {
    nice.latencyMs = 20;
    const tokens = await manager();
    const issuedBefore = nice.refreshToken;

    await Promise.all([tokens.token(AP), tokens.token(niceIn(ORG_A))]);

    expect(nice.maxInFlight).toBe(1);
    const [first, second] = refreshes();
    expect(first.get("refresh_token")).toBe(issuedBefore);
    expect(second.get("refresh_token")).toBe("rt-1");
    expect(storage.getItem(keys.refreshToken)).toBe(nice.refreshToken);
  });

  it("shares one refresh between concurrent requests for the same context", async () => {
    nice.latencyMs = 20;
    const tokens = await manager();

    const results = await Promise.all([tokens.token(AP), tokens.token(AP), tokens.token(AP)]);

    expect(new Set(results).size).toBe(1);
    expect(refreshes()).toHaveLength(1);
  });

  it("shares the first refresh when the organization is not known yet", async () => {
    nice.latencyMs = 20;
    const tokens = await manager();

    const [a, b] = await Promise.all([tokens.token(client), tokens.token(client)]);

    expect(a).toBe(b);
    expect(refreshes()).toHaveLength(1);
  });

  it("keeps the rotated refresh token before the promise settles, even when the rest of the answer is rejected", async () => {
    const tokens = await manager();
    nice.tokenOverride = () =>
      json(200, {
        access_token: nice.accessToken("inventaris", ORG_A),
        token_type: "Bearer",
        refresh_token: nice.rotate(),
        id_token: fakeJwt({ iss: nice.issuer, sub: "usr_x", aud: "someone-else", iat: 1, exp: 4102444800 }),
      });

    await expect(tokens.token(AP)).rejects.toMatchObject({ kind: "refused" });

    expect(storage.getItem(keys.refreshToken)).toBe(nice.refreshToken);
    nice.tokenOverride = undefined;
    await expect(tokens.token(AP)).resolves.toBeTruthy();
  });
});

describe("retry on a stale token", () => {
  it("gets a new token once and repeats the request once on token_stale", async () => {
    const tokens = await manager();
    let calls = 0;
    nice.service = () =>
      ++calls === 1 ? json(401, { error: "invalid_token", error_description: "token_stale" }) : json(200, { items: [] });

    const response = await tokens.call(AP, ITEMS);

    expect(response.status).toBe(200);
    expect(refreshes()).toHaveLength(2);
    expect(nice.serviceRequests[0].authorization).not.toBe(nice.serviceRequests[1].authorization);
  });

  it("returns the second 401 as it is", async () => {
    const tokens = await manager();
    nice.service = () => json(401, { error: "invalid_token", error_description: "token_stale" });

    expect((await tokens.call(AP, ITEMS)).status).toBe(401);
    expect(nice.serviceRequests).toHaveLength(2);
  });

  it("does not repeat other 401 answers", async () => {
    const tokens = await manager();
    nice.service = () => json(401, { error: "invalid_token", error_description: "wrong_audience" });

    expect((await tokens.call(AP, ITEMS)).status).toBe(401);
    expect(nice.serviceRequests).toHaveLength(1);
  });
});

describe("session end and temporary failures", () => {
  it("ends the session on invalid_grant and drops every token", async () => {
    const tokens = await manager();
    await tokens.token(AP);
    nice.refreshToken = "revoked-elsewhere";

    await expect(tokens.token(BR)).rejects.toMatchObject({ kind: "session_ended", code: "invalid_grant" });

    expect(ended).toHaveBeenCalledTimes(1);
    expect(tokens.snapshot()).toEqual([]);
    expect(storage.getItem(keys.refreshToken)).toBeNull();
  });

  it("keeps the old token and the session while NICE is down, then backs off", async () => {
    const tokens = await manager();
    const old = await tokens.token(AP);
    now += 10 * MINUTE;
    nice.tokenOverride = () => Promise.reject(new TypeError("Failed to fetch"));

    expect(await tokens.token(AP)).toBe(old);
    now += 4_999;
    expect(await tokens.token(AP)).toBe(old);
    expect(refreshes()).toHaveLength(2);
    now += 1;
    await tokens.token(AP);
    expect(refreshes()).toHaveLength(3);
    expect(ended).not.toHaveBeenCalled();
  });
});

// These tests tamper with the request on its way to the fake NICE, the way a
// client without nice:org or without the base scope would send it, and show
// that the fake refuses it or answers in the wrong organization.
describe("the fake NICE catches a client that", () => {
  const strip = (pattern: RegExp) => (form: URLSearchParams) => {
    form.set("scope", (form.get("scope") ?? "").split(" ").filter((s) => !pattern.test(s)).join(" "));
    return undefined;
  };

  it("drops nice:org: the switch to B/R is refused, because R is not a project of A", async () => {
    const tokens = await manager();
    await tokens.token(AP);
    nice.tokenOverride = strip(/^nice:org:/);

    await expect(tokens.token(BR)).rejects.toMatchObject({ code: "invalid_scope" });
    expect(nice.sessionOrg).toBe(ORG_A);
  });

  it("drops nice:org: the delegated token stays in the old organization", async () => {
    const tokens = await manager();
    await tokens.token(AP);
    nice.tokenOverride = strip(/^nice:org:/);

    expect(claimsOf(await tokens.token(niceIn(ORG_B))).nice_org_id).toBe(ORG_A);
  });

  it("drops the base scope: the next client token for userinfo is refused invalid_scope", async () => {
    const tokens = await manager();
    nice.tokenOverride = (form) =>
      form.get("scope") === BASE_SCOPE ? undefined : strip(/^(openid|profile|email|offline_access)$/)(form);
    await tokens.token(AP);

    await expect(tokens.token({ org: ORG_A })).rejects.toMatchObject({ kind: "refused", code: "invalid_scope" });
  });

  it("relies on nice:org in scope: a refresh without scope stays in the organization of the refresh token", async () => {
    const tokens = await manager();
    await tokens.token(BR);
    nice.tokenOverride = (form) => {
      form.delete("scope");
      return undefined;
    };

    expect(claimsOf(await tokens.token(AQ)).nice_org_id).toBe(ORG_B);
  });
});
