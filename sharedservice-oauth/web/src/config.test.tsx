import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { type ConfigResult, readConfig } from "./config";
import { ConfigErrorPage } from "./ui/ConfigErrorPage";

const ORIGIN = "http://localhost:5181";
const valid = {
  VITE_NICE_ISSUER: "http://localhost:8080",
  VITE_NICE_CLIENT_ID: "3f1d6c0e-ui",
  VITE_INVENTARIS_URL: "http://127.0.0.1:8081",
};

function errorsOf(result: ConfigResult) {
  return result.ok ? [] : result.errors;
}

describe("readConfig", () => {
  it("reads a complete configuration and fills the default URIs from the origin", () => {
    expect(readConfig(valid, ORIGIN)).toEqual({
      ok: true,
      config: {
        issuer: "http://localhost:8080",
        clientId: "3f1d6c0e-ui",
        redirectUri: "http://localhost:5181/callback",
        postLogoutRedirectUri: "http://localhost:5181/",
        inventarisUrl: "http://127.0.0.1:8081",
      },
    });
  });

  it("drops a trailing slash from the inventaris URL", () => {
    const result = readConfig({ ...valid, VITE_INVENTARIS_URL: "http://127.0.0.1:8081/" }, ORIGIN);

    expect(result.ok && result.config.inventarisUrl).toBe("http://127.0.0.1:8081");
  });

  it.each([
    ["missing", undefined],
    ["not a URL", "127.0.0.1:8081"],
    ["carrying a query", "http://127.0.0.1:8081/?x=1"],
  ])("names VITE_INVENTARIS_URL when it is %s", (_, url) => {
    expect(errorsOf(readConfig({ ...valid, VITE_INVENTARIS_URL: url }, ORIGIN)).map((e) => e.variable)).toEqual([
      "VITE_INVENTARIS_URL",
    ]);
  });

  it("names VITE_NICE_CLIENT_ID when the client id is empty", () => {
    expect(errorsOf(readConfig({ ...valid, VITE_NICE_CLIENT_ID: " " }, ORIGIN)).map((e) => e.variable)).toEqual([
      "VITE_NICE_CLIENT_ID",
    ]);
  });

  it("names VITE_NICE_ISSUER for a missing issuer or an http issuer that is not loopback", () => {
    for (const issuer of [undefined, "nice.example.com", "http://nice.example.com", "https://nice.example.com/?x=1"]) {
      expect(errorsOf(readConfig({ ...valid, VITE_NICE_ISSUER: issuer }, ORIGIN)).map((e) => e.variable)).toEqual([
        "VITE_NICE_ISSUER",
      ]);
    }
    expect(readConfig({ ...valid, VITE_NICE_ISSUER: "https://nice.example.com/" }, ORIGIN)).toMatchObject({
      ok: true,
      config: { issuer: "https://nice.example.com" },
    });
  });

  it("names VITE_REDIRECT_URI when it does not point at this UI's /callback", () => {
    for (const uri of ["http://localhost:5180/callback", "http://localhost:5181/login", "not a url"]) {
      expect(errorsOf(readConfig({ ...valid, VITE_REDIRECT_URI: uri }, ORIGIN)).map((e) => e.variable)).toEqual([
        "VITE_REDIRECT_URI",
      ]);
    }
  });

  it("names VITE_POST_LOGOUT_REDIRECT_URI when it is not a URL", () => {
    expect(
      errorsOf(readConfig({ ...valid, VITE_POST_LOGOUT_REDIRECT_URI: "/home" }, ORIGIN)).map((e) => e.variable),
    ).toEqual(["VITE_POST_LOGOUT_REDIRECT_URI"]);
  });

  it("reports every broken variable at once", () => {
    expect(errorsOf(readConfig({}, ORIGIN)).map((e) => e.variable)).toEqual([
      "VITE_NICE_ISSUER",
      "VITE_NICE_CLIENT_ID",
      "VITE_INVENTARIS_URL",
    ]);
  });
});

describe("ConfigErrorPage", () => {
  it("shows the name of each broken variable and no sign-in", () => {
    const html = renderToStaticMarkup(
      <ConfigErrorPage errors={errorsOf(readConfig({ ...valid, VITE_NICE_CLIENT_ID: "", VITE_INVENTARIS_URL: "x" }, ORIGIN))} />,
    );

    expect(html).toContain("<code>VITE_NICE_CLIENT_ID</code>");
    expect(html).toContain("<code>VITE_INVENTARIS_URL</code>");
    expect(html).not.toContain("Masuk");
  });
});
