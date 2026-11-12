import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Session, type SessionState } from "../session";
import { CLIENT_ID, ORG, POST_LOGOUT_URI, REDIRECT_URI, SUBJECT } from "../test/fake-nice";
import { MemoryStorage } from "../test/memory-storage";
import { Home } from "./Home";

const session = new Session(
  { issuer: "https://nice.test", clientId: CLIENT_ID, redirectUri: REDIRECT_URI, postLogoutRedirectUri: POST_LOGOUT_URI, services: [] },
  { storage: new MemoryStorage(), fetch: () => Promise.reject(new Error("offline")), navigate: () => {}, replaceUrl: () => {} },
);

function render(userinfo: Record<string, unknown>): string {
  const state: SessionState = { phase: "signed-in", identity: { userinfo, orgId: ORG }, tokens: [] };
  return renderToStaticMarkup(<Home session={session} state={state} />);
}

const emailCell = (markup: string) => /<dt>Email<\/dt><dd>([^<]*)<\/dd>/.exec(markup)?.[1];

describe("identity", () => {
  it("shows the email from email and leaves preferred_username to the userinfo JSON", () => {
    const markup = render({ sub: SUBJECT, name: "Siti Rahma", email: "siti@nusapala.id", preferred_username: "siti.lama" });

    expect(emailCell(markup)).toBe("siti@nusapala.id");
    expect(markup.split("siti.lama")).toHaveLength(2);
    expect(markup).toContain("<pre>");
    expect(markup).not.toContain("dari preferred_username");
    expect(markup).toContain(ORG);
    expect(markup).toContain("klaim access token client");
  });

  it("does not fall back to preferred_username when email is missing", () => {
    expect(emailCell(render({ sub: SUBJECT, preferred_username: "siti@nusapala.id" }))).toBe("-");
  });
});
