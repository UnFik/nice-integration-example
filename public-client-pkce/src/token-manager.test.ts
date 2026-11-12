import { beforeEach, describe, expect, it, vi } from "vitest";
import { OidcClient } from "./oidc";
import { BASE_SCOPE } from "./scope";
import { keys } from "./storage";
import { CLIENT_ID, FakeNice, fakeJwt, json, LOGIN_SCOPES, POST_LOGOUT_URI, REDIRECT_URI } from "./test/fake-nice";
import { MemoryStorage } from "./test/memory-storage";
import { CLIENT_KEY, type ServiceKey, TokenError, TokenManager, type TokenManagerOptions } from "./token-manager";

const MINUTE = 60_000;
const inventaris: ServiceKey = { slug: "inventaris" };
const pengiriman: ServiceKey = { slug: "pengiriman" };
const PROJECT = "prj_01a17b49-56bc-7632-a32f-d47311c068c1";

let now: number;
let nice: FakeNice;
let storage: MemoryStorage;
let ended: ReturnType<typeof vi.fn<(error: TokenError) => void>>;

beforeEach(() => {
  now = Date.parse("2026-11-12T03:00:00Z");
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
const claimsOf = (token: string) => JSON.parse(atob(token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")));

describe("token per service", () => {
  it("asks for the service with the base scope plus nice:aud, and nice:prj when a project is set", async () => {
    const tokens = await manager();

    await tokens.token(inventaris);
    await tokens.token({ slug: "inventaris", project: PROJECT });

    expect(refreshes().map((form) => form.get("scope"))).toEqual([
      "openid profile email offline_access nice:aud:inventaris",
      `openid profile email offline_access nice:aud:inventaris nice:prj:${PROJECT}`,
    ]);
    expect(refreshes()[0].get("client_id")).toBe(CLIENT_ID);
  });

  it("reuses the token for two calls within a minute", async () => {
    const tokens = await manager();
    nice.service = () => json(200, { items: [] });

    await tokens.call(inventaris, "http://127.0.0.1:8081/items");
    now += 59_000;
    await tokens.call(inventaris, "http://127.0.0.1:8081/items");

    expect(refreshes()).toHaveLength(1);
    expect(nice.serviceRequests).toHaveLength(2);
    expect(nice.serviceRequests[0].authorization).toBe(nice.serviceRequests[1].authorization);
    expect(claimsOf(nice.serviceRequests[0].authorization?.slice(7) ?? "").aud).toBe("inventaris");
  });

  it("renews after iat + 10 minutes + jitter", async () => {
    const tokens = await manager({ random: () => 0.5 });
    const first = await tokens.token(inventaris);

    now += 10 * MINUTE + 29_000;
    expect(await tokens.token(inventaris)).toBe(first);
    now += 1_000;
    const second = await tokens.token(inventaris);

    expect(second).not.toBe(first);
    expect(refreshes()).toHaveLength(2);
  });

  it("renews a token that is within 60 seconds of exp", async () => {
    nice.accessTtlSeconds = 5 * 60;
    const tokens = await manager();
    const first = await tokens.token(inventaris);

    now += 3 * MINUTE + 59_000;
    expect(await tokens.token(inventaris)).toBe(first);
    now += 1_000;

    expect(await tokens.token(inventaris)).not.toBe(first);
    expect(refreshes()).toHaveLength(2);
  });

  it("gets a second service through the refresh token", async () => {
    const tokens = await manager();

    await tokens.token(inventaris);
    const second = await tokens.token(pengiriman);

    expect(claimsOf(second).aud).toBe("pengiriman");
    expect(nice.tokenRequests.every((form) => form.get("grant_type") === "refresh_token")).toBe(true);
  });

  it("keeps a refused pair to its own entry", async () => {
    const tokens = await manager();
    nice.tokenOverride = (form) =>
      form.get("scope")?.includes("nice:aud:pengiriman")
        ? json(400, { error: "invalid_scope", error_description: "service is not active in the project" })
        : undefined;

    const refused = await tokens.token(pengiriman).catch((error: unknown) => error);
    const other = await tokens.token(inventaris);

    expect(refused).toBeInstanceOf(TokenError);
    expect(refused).toMatchObject({ kind: "refused", code: "invalid_scope" });
    expect(claimsOf(other).aud).toBe("inventaris");
    expect(ended).not.toHaveBeenCalled();
  });
});

describe("client token", () => {
  it.each([
    ["a service", inventaris],
    ["a service in a project", { slug: "inventaris", project: PROJECT }],
  ])("is still issued with the base scope only after a token for %s", async (_, service) => {
    const tokens = await manager();

    await tokens.token(service);
    const client = await tokens.token(CLIENT_KEY);

    expect(refreshes().at(-1)?.get("scope")).toBe(BASE_SCOPE);
    expect(claimsOf(client).aud).toBe(CLIENT_ID);
    expect(nice.grantScopes).toEqual(LOGIN_SCOPES);
    await expect(tokens.token(pengiriman)).resolves.toBeTruthy();
    expect(ended).not.toHaveBeenCalled();
  });

  it("is refused invalid_scope by NICE when a refresh drops the base scope", async () => {
    const tokens = await manager();
    nice.tokenOverride = (form) => {
      if (form.get("scope") !== BASE_SCOPE) form.set("scope", "nice:aud:inventaris");
      return undefined;
    };
    await tokens.token(inventaris);

    await expect(tokens.token(CLIENT_KEY)).rejects.toMatchObject({ kind: "refused", code: "invalid_scope" });
  });

  it("goes through the same refresh queue as the service tokens", async () => {
    nice.latencyMs = 20;
    const tokens = await manager();

    const [service, client, again] = await Promise.all([
      tokens.token(inventaris),
      tokens.token(CLIENT_KEY),
      tokens.token(CLIENT_KEY),
    ]);

    expect(claimsOf(service).aud).toBe("inventaris");
    expect(claimsOf(client).aud).toBe(CLIENT_ID);
    expect(again).toBe(client);
    expect(nice.maxInFlight).toBe(1);
    expect(refreshes().map((form) => form.get("scope"))).toEqual([
      `${BASE_SCOPE} nice:aud:inventaris`,
      BASE_SCOPE,
    ]);
  });

  it("reuses a held token under the same rules, then renews it with the base scope", async () => {
    const tokens = await manager();
    const login = nice.accessToken(CLIENT_ID);
    tokens.hold(CLIENT_KEY, login, 1800);

    now += 10 * MINUTE - 1_000;
    expect(await tokens.token(CLIENT_KEY)).toBe(login);
    expect(refreshes()).toHaveLength(0);
    now += 1_000;
    const renewed = await tokens.token(CLIENT_KEY);

    expect(renewed).not.toBe(login);
    expect(refreshes().map((form) => form.get("scope"))).toEqual([BASE_SCOPE]);
    expect(tokens.snapshot()).toEqual([expect.objectContaining({ key: "client", access: renewed })]);
    expect(tokens.snapshot()[0].service).toBeUndefined();
  });
});

describe("one refresh at a time", () => {
  it("sends the refreshes of two services one after the other, each with the latest refresh token", async () => {
    nice.latencyMs = 20;
    const tokens = await manager();
    const issuedBefore = nice.refreshToken;

    const [a, b] = await Promise.all([tokens.token(inventaris), tokens.token(pengiriman)]);

    expect(claimsOf(a).aud).toBe("inventaris");
    expect(claimsOf(b).aud).toBe("pengiriman");
    expect(nice.maxInFlight).toBe(1);
    const [first, second] = refreshes();
    expect(first.get("refresh_token")).toBe(issuedBefore);
    expect(second.get("refresh_token")).not.toBe(issuedBefore);
    expect(second.get("refresh_token")).toBe("rt-1");
    expect(storage.getItem(keys.refreshToken)).toBe(nice.refreshToken);
    expect(ended).not.toHaveBeenCalled();
  });

  it("shares one refresh between concurrent requests for the same pair", async () => {
    nice.latencyMs = 20;
    const tokens = await manager();

    const results = await Promise.all([tokens.token(inventaris), tokens.token(inventaris), tokens.token(inventaris)]);

    expect(new Set(results).size).toBe(1);
    expect(refreshes()).toHaveLength(1);
  });

  it("keeps the rotated refresh token before the promise settles, even when the rest of the answer is rejected", async () => {
    const tokens = await manager();
    nice.tokenOverride = () =>
      json(200, {
        access_token: nice.accessToken("inventaris"),
        token_type: "Bearer",
        refresh_token: nice.rotate(),
        id_token: fakeJwt({ iss: nice.issuer, sub: "usr_x", aud: "someone-else", iat: 1, exp: 4102444800 }),
      });

    await expect(tokens.token(inventaris)).rejects.toMatchObject({ kind: "refused" });

    expect(storage.getItem(keys.refreshToken)).toBe(nice.refreshToken);
    nice.tokenOverride = undefined;
    await expect(tokens.token(inventaris)).resolves.toBeTruthy();
    expect(ended).not.toHaveBeenCalled();
  });
});

describe("retry on a stale token", () => {
  it("gets a new token once and repeats the request once on token_stale", async () => {
    const tokens = await manager();
    let calls = 0;
    nice.service = () =>
      ++calls === 1
        ? json(401, { error: "invalid_token", error_description: "token_stale" })
        : json(200, { items: [{ id: "1" }] });

    const response = await tokens.call(inventaris, "http://127.0.0.1:8081/items");

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ items: [{ id: "1" }] });
    expect(refreshes()).toHaveLength(2);
    expect(nice.serviceRequests[0].authorization).not.toBe(nice.serviceRequests[1].authorization);
  });

  it("returns the second 401 as it is", async () => {
    const tokens = await manager();
    nice.service = () => json(401, { error: "invalid_token", error_description: "expired" });

    const response = await tokens.call(inventaris, "http://127.0.0.1:8081/items");

    expect(response.status).toBe(401);
    expect(await response.json()).toEqual({ error: "invalid_token", error_description: "expired" });
    expect(nice.serviceRequests).toHaveLength(2);
    expect(refreshes()).toHaveLength(2);
  });

  it("reads the WWW-Authenticate challenge when the body is not JSON", async () => {
    const tokens = await manager();
    let calls = 0;
    nice.service = () =>
      ++calls === 1
        ? new Response("stale", {
            status: 401,
            headers: { "WWW-Authenticate": 'Bearer error="invalid_token", error_description="token_stale"' },
          })
        : json(200, {});

    expect((await tokens.call(inventaris, "http://127.0.0.1:8081/items")).status).toBe(200);
    expect(nice.serviceRequests).toHaveLength(2);
  });

  it("does not repeat other 401 answers", async () => {
    const tokens = await manager();
    nice.service = () => json(401, { error: "invalid_token", error_description: "wrong_audience" });

    expect((await tokens.call(inventaris, "http://127.0.0.1:8081/items")).status).toBe(401);
    expect(nice.serviceRequests).toHaveLength(1);
    expect(refreshes()).toHaveLength(1);
  });
});

describe("session end and temporary failures", () => {
  it("ends the session on invalid_grant and drops every token", async () => {
    const tokens = await manager();
    await tokens.token(inventaris);
    nice.refreshToken = "revoked-elsewhere";

    await expect(tokens.token(pengiriman)).rejects.toMatchObject({ kind: "session_ended", code: "invalid_grant" });

    expect(ended).toHaveBeenCalledTimes(1);
    expect(tokens.snapshot()).toEqual([]);
    expect(storage.getItem(keys.refreshToken)).toBeNull();
    const sent = nice.tokenRequests.length;
    await expect(tokens.token(inventaris)).rejects.toMatchObject({ kind: "session_ended" });
    expect(nice.tokenRequests).toHaveLength(sent);
  });

  it("ends the session on invalid_client, also behind a WWW-Authenticate challenge", async () => {
    const tokens = await manager();
    nice.tokenOverride = () => json(401, { error: "invalid_client" }, { "WWW-Authenticate": 'Basic realm="nice"' });

    await expect(tokens.token(inventaris)).rejects.toMatchObject({ kind: "session_ended", code: "invalid_client" });
    expect(ended).toHaveBeenCalledTimes(1);
  });

  it.each([
    ["the network fails", () => Promise.reject(new TypeError("Failed to fetch"))],
    ["NICE answers 503", () => json(503, { error: "temporarily_unavailable" })],
    ["NICE answers 429", () => json(429, { error: "rate_limited" })],
  ])("keeps the old token and the session when %s, then backs off 5 to 60 seconds", async (_, failure) => {
    const tokens = await manager();
    const old = await tokens.token(inventaris);
    now += 10 * MINUTE;
    nice.tokenOverride = failure as () => Promise<Response>;

    expect(await tokens.token(inventaris)).toBe(old);
    expect(refreshes()).toHaveLength(2);
    now += 4_999;
    expect(await tokens.token(inventaris)).toBe(old);
    expect(refreshes()).toHaveLength(2);
    now += 1;
    expect(await tokens.token(inventaris)).toBe(old);
    expect(refreshes()).toHaveLength(3);
    now += 9_999;
    await tokens.token(inventaris);
    expect(refreshes()).toHaveLength(3);
    now += 1;
    await tokens.token(inventaris);
    expect(refreshes()).toHaveLength(4);

    for (let i = 0; i < 6; i++) {
      now += 60_000;
      await tokens.token(inventaris);
    }
    expect(refreshes()).toHaveLength(10);
    expect(ended).not.toHaveBeenCalled();

    nice.tokenOverride = undefined;
    now += 60_000;
    expect(await tokens.token(inventaris)).not.toBe(old);
    expect(storage.getItem(keys.refreshToken)).toBe(nice.refreshToken);
  });

  it("calls the service with the old token while NICE is down", async () => {
    const tokens = await manager();
    nice.service = () => json(200, {});
    const old = await tokens.token(inventaris);
    now += 11 * MINUTE;
    nice.tokenOverride = () => Promise.reject(new TypeError("Failed to fetch"));

    const response = await tokens.call(inventaris, "http://127.0.0.1:8081/items");

    expect(response.status).toBe(200);
    expect(nice.serviceRequests[0].authorization).toBe(`Bearer ${old}`);
    expect(ended).not.toHaveBeenCalled();
  });

  it("fails without ending the session when there is no valid token and NICE is down", async () => {
    const tokens = await manager();
    nice.tokenOverride = () => Promise.reject(new TypeError("Failed to fetch"));

    await expect(tokens.token(inventaris)).rejects.toMatchObject({ kind: "temporary" });
    await expect(tokens.token(inventaris)).rejects.toMatchObject({ kind: "temporary" });

    expect(refreshes()).toHaveLength(1);
    expect(ended).not.toHaveBeenCalled();
    expect(storage.getItem(keys.refreshToken)).toBe(nice.refreshToken);
  });
});
