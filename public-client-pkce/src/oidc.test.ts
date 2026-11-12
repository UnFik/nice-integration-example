import { beforeEach, describe, expect, it } from "vitest";
import { InsecureIssuerError, issuerTransport, LoginError, OidcClient } from "./oidc";
import { BASE_SCOPE } from "./scope";
import { keys } from "./storage";
import { CLIENT_ID, FakeNice, POST_LOGOUT_URI, REDIRECT_URI, SUBJECT } from "./test/fake-nice";
import { MemoryStorage } from "./test/memory-storage";

let nice: FakeNice;
let storage: MemoryStorage;

beforeEach(() => {
  nice = new FakeNice();
  storage = new MemoryStorage();
});

function client(issuer = nice.issuer, fetchImpl: typeof fetch = nice.fetch) {
  return OidcClient.discover({
    issuer,
    clientId: CLIENT_ID,
    redirectUri: REDIRECT_URI,
    postLogoutRedirectUri: POST_LOGOUT_URI,
    fetch: fetchImpl,
  });
}

async function s256(verifier: string): Promise<string> {
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier)));
  return btoa(String.fromCharCode(...digest)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

describe("authorization request", () => {
  it("uses PKCE S256, a random state and nonce, and the base scope", async () => {
    const oidc = await client();
    const url = new URL(await oidc.authorizationUrl(storage));
    const transaction = JSON.parse(storage.getItem(keys.login) ?? "{}");

    expect(url.origin + url.pathname).toBe(`${nice.issuer}/oauth/authorize`);
    expect(url.searchParams.get("response_type")).toBe("code");
    expect(url.searchParams.get("client_id")).toBe(CLIENT_ID);
    expect(url.searchParams.get("redirect_uri")).toBe(REDIRECT_URI);
    expect(url.searchParams.get("scope")).toBe(BASE_SCOPE);
    expect(BASE_SCOPE).toBe("openid profile email offline_access");
    expect(url.searchParams.get("code_challenge_method")).toBe("S256");
    expect(url.searchParams.get("code_challenge")).toBe(await s256(transaction.verifier));
    expect(url.searchParams.get("state")).toBe(transaction.state);
    expect(url.searchParams.get("nonce")).toBe(transaction.nonce);
    expect(transaction.state).not.toBe(transaction.nonce);

    const again = new URL(await oidc.authorizationUrl(new MemoryStorage()));
    expect(again.searchParams.get("state")).not.toBe(url.searchParams.get("state"));
    expect(again.searchParams.get("nonce")).not.toBe(url.searchParams.get("nonce"));
  });
});

describe("callback", () => {
  it("exchanges the code with the verifier and removes the transaction", async () => {
    const oidc = await client();
    const callback = nice.authorize(await oidc.authorizationUrl(storage));

    const outcome = await oidc.completeLogin(new URL(callback), storage);

    expect(outcome.kind).toBe("signed-in");
    if (outcome.kind !== "signed-in") return;
    expect(outcome.subject).toBe(SUBJECT);
    expect(outcome.refreshToken).toBe(nice.refreshToken);
    expect(storage.getItem(keys.login)).toBeNull();
    const exchange = nice.tokenRequests[0];
    expect(exchange.get("grant_type")).toBe("authorization_code");
    expect(exchange.get("code_verifier")).toBeTruthy();
    expect(nice.grantScopes).toEqual(["openid", "profile", "email", "offline_access"]);
  });

  it("rejects a state it did not create, without exchanging the code", async () => {
    const oidc = await client();
    const callback = new URL(nice.authorize(await oidc.authorizationUrl(storage)));
    callback.searchParams.set("state", "forged-state");

    await expect(oidc.completeLogin(callback, storage)).rejects.toBeInstanceOf(LoginError);

    expect(nice.tokenRequests).toHaveLength(0);
    expect(storage.getItem(keys.login)).toBeNull();
  });

  it("rejects a state that was already used", async () => {
    const oidc = await client();
    const callback = new URL(nice.authorize(await oidc.authorizationUrl(storage)));
    await oidc.completeLogin(callback, storage);

    await expect(oidc.completeLogin(callback, storage)).rejects.toThrow(/sudah dipakai/);

    expect(nice.tokenRequests).toHaveLength(1);
  });

  it("rejects a callback when no login was started in this tab", async () => {
    const oidc = await client();

    await expect(oidc.completeLogin(new URL(`${REDIRECT_URI}?code=c&state=s`), storage)).rejects.toBeInstanceOf(
      LoginError,
    );
    expect(nice.tokenRequests).toHaveLength(0);
  });

  it("rejects an ID token whose nonce is not the one sent", async () => {
    const oidc = await client();
    const callback = nice.authorize(await oidc.authorizationUrl(storage));
    nice.idTokenNonce = "another-nonce";

    await expect(oidc.completeLogin(new URL(callback), storage)).rejects.toThrow(/nonce/);
    expect(storage.getItem(keys.login)).toBeNull();
  });

  it("asks to start again on authentication_expired", async () => {
    const oidc = await client();
    const state = new URL(await oidc.authorizationUrl(storage)).searchParams.get("state");
    const callback = new URL(REDIRECT_URI);
    callback.search = new URLSearchParams({
      error: "temporarily_unavailable",
      error_description: "authentication_expired",
      state: state ?? "",
    }).toString();

    await expect(oidc.completeLogin(callback, storage)).resolves.toEqual({ kind: "restart" });
    expect(nice.tokenRequests).toHaveLength(0);
    expect(storage.getItem(keys.login)).toBeNull();
  });

  it("reports other authorization errors", async () => {
    const oidc = await client();
    const state = new URL(await oidc.authorizationUrl(storage)).searchParams.get("state");
    const callback = new URL(`${REDIRECT_URI}?error=access_denied&state=${state}`);

    await expect(oidc.completeLogin(callback, storage)).rejects.toThrow(/access_denied/);
  });

  it("fails when NICE gives no refresh token", async () => {
    const oidc = await client();
    nice.issueRefreshTokenOnLogin = false;
    const callback = nice.authorize(await oidc.authorizationUrl(storage));

    await expect(oidc.completeLogin(new URL(callback), storage)).rejects.toThrow(/refresh token/);
  });
});

describe("issuer transport", () => {
  it("allows insecure requests only for an http issuer on a loopback host", () => {
    expect(issuerTransport(new URL("https://nice.example.com"))).toEqual({ allowInsecure: false });
    for (const issuer of ["http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"]) {
      expect(issuerTransport(new URL(issuer))).toEqual({ allowInsecure: true });
    }
  });

  it("rejects an http issuer on any other host", () => {
    for (const issuer of ["http://nice.example.com", "http://10.0.0.5:8080", "http://localhost.example.com"]) {
      expect(() => issuerTransport(new URL(issuer))).toThrow(InsecureIssuerError);
    }
  });

  it("refuses discovery from a non-loopback http issuer before sending anything", async () => {
    const seen: string[] = [];
    const record: typeof fetch = async (input) => {
      seen.push(String(input));
      return new Response("{}");
    };

    await expect(client("http://nice.example.com", record)).rejects.toBeInstanceOf(InsecureIssuerError);
    expect(seen).toEqual([]);
  });

  it("talks to a loopback http issuer", async () => {
    const local = new FakeNice(Date.now, "http://localhost:8080");

    const oidc = await client(local.issuer, local.fetch);

    expect(oidc.as.issuer).toBe("http://localhost:8080");
  });
});
