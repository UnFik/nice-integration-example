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
    const markup = await screen("http://localhost:5181/", (backend) => (backend.signedIn = false));

    expect(markup).toMatch(/>Masuk</);
    expect(markup).toMatch(/Masuk dengan akun lain/);
  });

  it("says the user signed out after Keluar", async () => {
    const backend = new FakeBackend();
    const session = new Session({ fetch: backend.fetch, navigate: () => {}, replaceUrl: () => {} });
    await session.start("http://localhost:5181/");
    await session.logout();

    expect(renderToStaticMarkup(<App session={session} />)).toMatch(/Anda sudah keluar dari nadi/);
  });

  it("explains access_denied and offers another account", async () => {
    const markup = await screen("http://localhost:5181/?login_error=access_denied");

    expect(markup).toMatch(/Service tidak tersedia/);
    expect(markup).toMatch(/Tidak ada organisasi Anda yang memakai nadi/);
    expect(markup).toMatch(/Masuk dengan akun lain/);
  });

  it("offers Masuk again after a callback that did not belong to this browser", async () => {
    const markup = await screen("http://localhost:5181/?login_error=state_mismatch");

    expect(markup).toMatch(/Login gagal/);
    expect(markup).toMatch(/Masuk lagi/);
  });

  it("shows Sesi berakhir with Masuk", async () => {
    const markup = await screen("http://localhost:5181/", (backend) => (backend.ended = true));

    expect(markup).toMatch(/Sesi berakhir/);
    expect(markup).toMatch(/>Masuk</);
  });

  it("shows the home page with the devices when signed in", async () => {
    const markup = await screen("http://localhost:5181/");

    expect(markup).toContain("Sensor Suhu Gudang");
    expect(markup).toMatch(/Keluar/);
  });
});
