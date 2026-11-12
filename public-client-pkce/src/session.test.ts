import { beforeEach, describe, expect, it } from "vitest";
import type { AppConfig } from "./config";
import { BASE_SCOPE } from "./scope";
import { Session } from "./session";
import { keys } from "./storage";
import { CLIENT_ID, EMAIL, FakeNice, json, ORG, POST_LOGOUT_URI, REDIRECT_URI, SUBJECT } from "./test/fake-nice";
import { MemoryStorage } from "./test/memory-storage";

let nice: FakeNice;
let storage: MemoryStorage;
let navigations: string[];
let replaced: string[];

const service = { slug: "inventaris", endpoint: "http://127.0.0.1:8081", path: "/items" };

beforeEach(() => {
  nice = new FakeNice();
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
    services: [service],
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

describe("login", () => {
  it("signs in from the callback, strips the URL, and keeps only the refresh and ID token", async () => {
    const current = await signedIn();
    const state = current.getState();

    expect(replaced).toEqual(["/"]);
    expect(state.phase).toBe("signed-in");
    expect(state.identity).toMatchObject({
      userinfo: { sub: SUBJECT, name: "Siti Rahma", email: EMAIL },
      orgId: ORG,
    });
    expect(storage.getItem(keys.refreshToken)).toBe(nice.refreshToken);
    expect(storage.getItem(keys.idToken)).toBe(state.idToken);
    expect(storedKeys()).toEqual([keys.idToken, keys.refreshToken].sort());
    const stored = storedKeys().map((key) => storage.getItem(key ?? ""));
    for (const held of state.tokens) expect(stored).not.toContain(held.access);
  });

  it("asks userinfo with the access token of the code exchange as the client token", async () => {
    const current = await signedIn();
    const client = current.getState().tokens.find((held) => !held.service);

    expect(client?.claims?.aud).toBe(CLIENT_ID);
    expect(refreshes()).toHaveLength(0);
    expect(nice.userinfoRequests).toEqual([{ authorization: `Bearer ${client?.access}` }]);
  });

  it("starts a new authorization request without an error on authentication_expired", async () => {
    const first = session();
    await first.resume();
    await first.signIn();
    const state = new URL(navigations[0]).searchParams.get("state");
    navigations = [];

    const current = session();
    await current.completeLogin(
      `${REDIRECT_URI}?error=temporarily_unavailable&error_description=authentication_expired&state=${state}`,
    );

    expect(current.getState().phase).toBe("redirecting");
    expect(navigations).toHaveLength(1);
    expect(new URL(navigations[0]).pathname).toBe("/oauth/authorize");
    expect(new URL(navigations[0]).searchParams.get("state")).not.toBe(state);
  });

  it("shows a failed login for a state it did not create", async () => {
    const current = session();

    await current.completeLogin(`${REDIRECT_URI}?code=code-1&state=forged`);

    expect(current.getState()).toMatchObject({ phase: "login-failed" });
    expect(nice.tokenRequests).toHaveLength(0);
  });
});

describe("reload", () => {
  it("stays signed in from sessionStorage without a redirect", async () => {
    await signedIn();

    const reloaded = session();
    await reloaded.resume();

    expect(reloaded.getState().phase).toBe("signed-in");
    expect(navigations).toEqual([]);
    nice.service = () => json(200, { items: [] });
    expect((await reloaded.call(service)).status).toBe(200);
    expect(nice.tokenRequests.at(-1)?.get("grant_type")).toBe("refresh_token");
  });

  it("fetches the identity again with one base-scope refresh and userinfo after a service was called", async () => {
    const current = await signedIn();
    nice.service = () => json(200, { items: [] });
    await current.call(service);
    const refreshesBefore = refreshes().length;
    const userinfoBefore = nice.userinfoRequests.length;
    expect(storedKeys()).toEqual([keys.idToken, keys.refreshToken].sort());

    const reloaded = session();
    await reloaded.resume();

    const state = reloaded.getState();
    expect(state.identityError).toBeUndefined();
    expect(state.identity).toEqual({
      userinfo: { sub: SUBJECT, name: "Siti Rahma", preferred_username: EMAIL, email: EMAIL, email_verified: false },
      orgId: ORG,
    });
    const newRefreshes = refreshes().slice(refreshesBefore);
    expect(newRefreshes.map((form) => form.get("scope"))).toEqual([BASE_SCOPE]);
    const client = state.tokens.find((held) => !held.service);
    expect(client?.claims?.aud).toBe(CLIENT_ID);
    expect(nice.userinfoRequests.slice(userinfoBefore)).toEqual([{ authorization: `Bearer ${client?.access}` }]);
    expect(navigations).toEqual([]);
    expect(storedKeys()).toEqual([keys.idToken, keys.refreshToken].sort());
  });

  it("ends the session when the refresh for the identity is refused invalid_grant", async () => {
    await signedIn();
    nice.refreshToken = "revoked";

    const reloaded = session();
    await reloaded.resume();

    expect(reloaded.getState()).toMatchObject({ phase: "ended", tokens: [] });
    expect(reloaded.getState().identityError).toBeUndefined();
    expect(storage.length).toBe(0);
  });

  it("is signed out in a tab without the session", async () => {
    const fresh = session();
    await fresh.resume();

    expect(fresh.getState().phase).toBe("signed-out");
  });
});

describe("session end", () => {
  it("shows Sesi berakhir and clears the session when NICE answers invalid_grant", async () => {
    const current = await signedIn();
    nice.refreshToken = "revoked";

    await expect(current.call(service)).rejects.toMatchObject({ kind: "session_ended" });

    expect(current.getState()).toMatchObject({ phase: "ended", tokens: [] });
    expect(storage.length).toBe(0);
  });
});

describe("logout", () => {
  it.each(["ok", "error", "network"] as const)(
    "revokes the refresh token, empties sessionStorage, and goes to end-session (revoke: %s)",
    async (answer) => {
      const current = await signedIn();
      const refreshToken = storage.getItem(keys.refreshToken);
      const idToken = storage.getItem(keys.idToken);
      nice.revokeAnswer = answer;

      await current.logout();

      expect(nice.revokeRequests).toHaveLength(1);
      expect(nice.revokeRequests[0].get("token")).toBe(refreshToken);
      expect(nice.revokeRequests[0].get("token_type_hint")).toBe("refresh_token");
      expect(nice.revokeRequests[0].get("client_id")).toBe(CLIENT_ID);
      expect(storage.length).toBe(0);
      expect(navigations).toHaveLength(1);
      const endSession = new URL(navigations[0]);
      expect(endSession.origin + endSession.pathname).toBe(`${nice.issuer}/oauth/end_session`);
      expect(endSession.searchParams.get("id_token_hint")).toBe(idToken);
      expect(endSession.searchParams.get("post_logout_redirect_uri")).toBe(POST_LOGOUT_URI);
    },
  );

  it("revokes the refresh token left by a refresh that was running", async () => {
    const current = await signedIn();
    nice.latencyMs = 20;
    nice.service = () => json(200, {});

    const call = current.call(service);
    await new Promise((resolve) => setTimeout(resolve, 5));
    await current.logout();
    await call.catch(() => undefined);

    expect(nice.revokeRequests[0].get("token")).toBe(nice.refreshToken);
    expect(storage.length).toBe(0);
  });
});
