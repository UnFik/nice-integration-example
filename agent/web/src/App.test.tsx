import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { Session } from "./session";
import { FakeBackend } from "./test/fake-backend";

async function screen(href: string, setup: (backend: FakeBackend) => void = () => {}): Promise<string> {
  const backend = new FakeBackend();
  setup(backend);
  const session = new Session({ fetch: backend.fetch, navigate: () => {}, replaceUrl: () => {} });
  await session.start(href);
  return renderToStaticMarkup(<App session={session} />);
}

describe("App", () => {
  it("shows Masuk without a session", async () => {
    const markup = await screen("http://localhost:5182/", (backend) => (backend.signedIn = false));

    expect(markup).toMatch(/>Masuk</);
    expect(markup).toMatch(/tidak memegang token/);
  });

  it("shows Sesi berakhir with Masuk", async () => {
    const markup = await screen("http://localhost:5182/", (backend) => (backend.ended = true));

    expect(markup).toMatch(/Sesi berakhir/);
  });

  it("explains access_denied from the callback", async () => {
    expect(await screen("http://localhost:5182/?login_error=access_denied")).toMatch(/Agen tidak tersedia/);
  });

  it("shows only the conversations of the user, with the fallback title, and the account menu", async () => {
    const markup = await screen("http://localhost:5182/", (backend) => {
      backend.conversations = backend.conversations.slice(0, 2);
    });

    expect([...markup.matchAll(/data-conversation="([^"]+)"/g)].map((match) => match[1])).toEqual(["c2", "c1"]);
    expect(markup).toContain("Daftar perangkat");
    expect(markup).toContain("Percakapan baru");
    expect(markup).toMatch(/Menu akun/);
  });
});
