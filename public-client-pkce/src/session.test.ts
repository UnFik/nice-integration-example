import { beforeEach, describe, expect, it } from "vitest";
import type { AppConfig } from "./config";
import { Session } from "./session";
import { keys } from "./storage";
import { CLIENT_ID, FakeNice, json, ORG, POST_LOGOUT_URI, REDIRECT_URI, SUBJECT } from "./test/fake-nice";
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

describe("login", () => {
  it("signs in from the callback, strips the URL, and keeps only the refresh and ID token", async () => {
    const current = await signedIn();
    const state = current.getState();

    expect(replaced).toEqual(["/"]);
    expect(state.phase).toBe("signed-in");
    expect(state.identity).toMatchObject({ userinfo: { sub: SUBJECT, name: "Siti Rahma" }, orgId: ORG });
    expect(state.loginAccessToken).toBeTruthy();
    expect(storage.getItem(keys.refreshToken)).toBe(nice.refreshToken);
    expect(storage.getItem(keys.idToken)).toBe(state.idToken);
    expect(storage.getItem(keys.login)).toBeNull();
    const stored = [...Array(storage.length).keys()].map((i) => storage.getItem(storage.key(i) ?? ""));
    expect(stored).not.toContain(state.loginAccessToken);
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
  it("stays signed in from sessionStorage without going to NICE", async () => {
    await signedIn();

    const reloaded = session();
    await reloaded.resume();

    expect(reloaded.getState().phase).toBe("signed-in");
    expect(reloaded.getState().loginAccessToken).toBeUndefined();
    expect(reloaded.getState().identity?.userinfo.sub).toBe(SUBJECT);
    expect(navigations).toEqual([]);
    nice.service = () => json(200, { items: [] });
    expect((await reloaded.call(service)).status).toBe(200);
    expect(nice.tokenRequests.at(-1)?.get("grant_type")).toBe("refresh_token");
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

    expect(current.getState()).toMatchObject({ phase: "ended", serviceTokens: [] });
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
