import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Session, type SessionState } from "../session";
import { CLIENT_ID, INVENTARIS_URL, ORG_A, POST_LOGOUT_URI, PRJ_P, REDIRECT_URI, SUBJECT } from "../test/fake-nice";
import { MemoryStorage } from "../test/memory-storage";
import { Home } from "./Home";

const session = new Session(
  { issuer: "https://nice.test", clientId: CLIENT_ID, redirectUri: REDIRECT_URI, postLogoutRedirectUri: POST_LOGOUT_URI, inventarisUrl: INVENTARIS_URL },
  { storage: new MemoryStorage(), fetch: () => Promise.reject(new Error("offline")), navigate: () => {}, replaceUrl: () => {} },
);

const signedIn: SessionState = {
  phase: "signed-in",
  identity: { userinfo: { sub: SUBJECT, name: "Siti Rahma", email: "siti@nusapala.id" }, orgId: ORG_A },
  tokens: [],
  active: { org: ORG_A },
  contexts: {
    currentOrganizationId: ORG_A,
    organizations: [
      { id: ORG_A, name: "Nusapala", slug: "nusapala", projects: [{ id: PRJ_P, name: "Parkir", slug: "parkir", services: ["inventaris"] }], projectsTruncated: false },
    ],
  },
};

const render = (state: SessionState) => renderToStaticMarkup(<Home session={session} state={state} />);

describe("Home", () => {
  it("shows the identity from userinfo and the organization from the client token", () => {
    const markup = render(signedIn);

    expect(markup).toMatch(/<dt>Email<\/dt><dd>siti@nusapala.id<\/dd>/);
    expect(markup).toContain(ORG_A);
  });

  it("shows the chooser from /me/contexts and asks to choose a project before listing items", () => {
    const markup = render(signedIn);

    expect(markup).toContain(`data-context="${ORG_A}/${PRJ_P}"`);
    expect(markup).toMatch(/Pilih project/);
  });

  it("shows a refused switch and keeps the chooser", () => {
    const markup = render({ ...signedIn, active: { org: ORG_A, project: PRJ_P }, switchError: "NICE menolak: invalid_scope" });

    expect(markup).toContain("NICE menolak: invalid_scope");
    expect(markup).toContain(`data-context="${ORG_A}/${PRJ_P}"`);
  });

  it("lists the items of the active context", () => {
    const markup = render({
      ...signedIn,
      active: { org: ORG_A, project: PRJ_P },
      items: [{ id: "itm-1", name: "Kabel UTP Cat6", quantity: 12 }],
    });

    expect(markup).toMatch(/Kabel UTP Cat6<\/td><td>12<\/td>/);
  });
});
