import { beforeEach, describe, expect, it } from "vitest";
import { ContextsError, loadContexts } from "./contexts";
import { OidcClient } from "./oidc";
import { keys } from "./storage";
import {
  CLIENT_ID,
  FakeNice,
  fakeJwt,
  json,
  ORG_A,
  ORG_B,
  ORG_C,
  POST_LOGOUT_URI,
  PRJ_P,
  PRJ_Q,
  PRJ_R,
  REDIRECT_URI,
} from "./test/fake-nice";
import { MemoryStorage } from "./test/memory-storage";
import { TokenManager } from "./token-manager";

let nice: FakeNice;
let tokens: TokenManager;

beforeEach(async () => {
  nice = new FakeNice();
  const storage = new MemoryStorage();
  storage.setItem(keys.refreshToken, nice.refreshToken);
  const oidc = await OidcClient.discover({
    issuer: nice.issuer,
    clientId: CLIENT_ID,
    redirectUri: REDIRECT_URI,
    postLogoutRedirectUri: POST_LOGOUT_URI,
    fetch: nice.fetch,
  });
  tokens = new TokenManager({ storage, grant: oidc.refreshGrant, fetch: nice.fetch, random: () => 0 });
});

const claimsOf = (authorization: string | null) =>
  JSON.parse(atob((authorization ?? "").slice(7).split(".")[1].replace(/-/g, "+").replace(/_/g, "/")));

describe("loadContexts", () => {
  it("reads /me/contexts with a delegated token of the organization, without naming a service", async () => {
    const contexts = await loadContexts(tokens, nice.issuer, ORG_B);

    expect(nice.contextsRequests).toHaveLength(1);
    const request = new URL(nice.contextsRequests[0].url);
    expect(request.searchParams.has("service")).toBe(false);
    expect(request.searchParams.get("page_size")).toBe("50");
    expect(claimsOf(nice.contextsRequests[0].authorization)).toMatchObject({
      aud: "nice",
      nice_org_id: ORG_B,
      perms: ["nice.me.contexts.read"],
    });
    expect(contexts.currentOrganizationId).toBe(ORG_B);
    expect(contexts.organizations.map((org) => [org.id, org.projects.map((p) => p.id)])).toEqual([
      [ORG_A, [PRJ_P, PRJ_Q]],
      [ORG_B, [PRJ_R]],
      [ORG_C, []],
    ]);
    expect(contexts.organizations[0].projects[0]).toEqual({ id: PRJ_P, name: "Parkir", slug: "parkir", services: ["inventaris"] });
  });

  it("follows next_page_token until the last page", async () => {
    nice.contextsPageSize = 1;

    const contexts = await loadContexts(tokens, nice.issuer, ORG_A);

    expect(nice.contextsRequests).toHaveLength(3);
    expect(new URL(nice.contextsRequests[1].url).searchParams.get("page_token")).toBe(ORG_A);
    expect(contexts.organizations.map((org) => org.id)).toEqual([ORG_A, ORG_B, ORG_C]);
  });

  it("reports the error_code NICE answers", async () => {
    nice.tokenOverride = (form) =>
      form.get("scope")?.includes("nice:aud:nice")
        ? json(200, {
            access_token: fakeJwt({ aud: "nice", nice_org_id: ORG_A, iat: 1, exp: 4102444800 }),
            token_type: "Bearer",
            refresh_token: nice.rotate(),
          })
        : undefined;

    const refused = await loadContexts(tokens, nice.issuer, ORG_A).catch((error: unknown) => error);

    expect(refused).toBeInstanceOf(ContextsError);
    expect(refused).toMatchObject({ status: 403, code: "delegation_not_allowed" });
  });
});
