import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { type ConfigResult, readConfig } from "./config";
import { ConfigErrorPage } from "./ui/ConfigErrorPage";

const ORIGIN = "http://localhost:5180";
const valid = {
  VITE_NICE_ISSUER: "http://localhost:8080",
  VITE_NICE_CLIENT_ID: "3f1d6c0e-spa",
  VITE_SERVICES: '[{"slug":"inventaris","endpoint":"http://127.0.0.1:8081","path":"/items"}]',
};

function errorsOf(result: ConfigResult) {
  return result.ok ? [] : result.errors;
}

describe("readConfig", () => {
  it("reads a complete configuration and fills the default URIs from the origin", () => {
    const result = readConfig(valid, ORIGIN);

    expect(result).toEqual({
      ok: true,
      config: {
        issuer: "http://localhost:8080",
        clientId: "3f1d6c0e-spa",
        redirectUri: "http://localhost:5180/callback",
        postLogoutRedirectUri: "http://localhost:5180/",
        services: [{ slug: "inventaris", endpoint: "http://127.0.0.1:8081", path: "/items" }],
      },
    });
  });

  it("keeps an optional project", () => {
    const project = "prj_01a17b49-56bc-7632-a32f-d47311c068c1";
    const result = readConfig(
      { ...valid, VITE_SERVICES: `[{"slug":"inventaris","endpoint":"http://127.0.0.1:8081/","path":"/items","project":"${project}"}]` },
      ORIGIN,
    );

    expect(result.ok && result.config.services[0]).toEqual({
      slug: "inventaris",
      endpoint: "http://127.0.0.1:8081",
      path: "/items",
      project,
    });
  });

  it("names VITE_NICE_CLIENT_ID when the client id is empty or missing", () => {
    for (const env of [{ ...valid, VITE_NICE_CLIENT_ID: "" }, { ...valid, VITE_NICE_CLIENT_ID: "  " }, { ...valid, VITE_NICE_CLIENT_ID: undefined }]) {
      const result = readConfig(env, ORIGIN);
      expect(result.ok).toBe(false);
      expect(errorsOf(result).map((error) => error.variable)).toEqual(["VITE_NICE_CLIENT_ID"]);
    }
  });

  it.each([
    ["missing", undefined, /wajib diisi/],
    ["not JSON", "[{slug:inventaris}]", /bukan JSON/],
    ["not an array", '{"slug":"inventaris"}', /array JSON/],
    ["an entry that is not an object", '["inventaris"]', /entri 0 harus berupa objek/],
    ["a bad slug", '[{"slug":"Inventaris!","endpoint":"http://127.0.0.1:8081","path":"/items"}]', /entri 0: slug/],
    ["a bad endpoint", '[{"slug":"inventaris","endpoint":"127.0.0.1:8081","path":"/items"}]', /endpoint/],
    ["a path without a leading slash", '[{"slug":"inventaris","endpoint":"http://127.0.0.1:8081","path":"items"}]', /path/],
    ["a bad project", '[{"slug":"inventaris","endpoint":"http://127.0.0.1:8081","path":"/items","project":"parkir"}]', /project/],
    ["an unknown field", '[{"slug":"inventaris","endpoint":"http://127.0.0.1:8081","path":"/items","projet":"x"}]', /projet/],
  ])("names VITE_SERVICES when it is %s", (_, services, message) => {
    const result = readConfig({ ...valid, VITE_SERVICES: services }, ORIGIN);

    expect(result.ok).toBe(false);
    const errors = errorsOf(result);
    expect(errors.map((error) => error.variable)).toEqual(["VITE_SERVICES"]);
    expect(errors[0].message).toMatch(message);
  });

  it("names VITE_NICE_ISSUER for a missing issuer or an http issuer that is not loopback", () => {
    for (const issuer of [undefined, "nice.example.com", "http://nice.example.com", "https://nice.example.com/?x=1"]) {
      const result = readConfig({ ...valid, VITE_NICE_ISSUER: issuer }, ORIGIN);
      expect(errorsOf(result).map((error) => error.variable)).toEqual(["VITE_NICE_ISSUER"]);
    }
    expect(readConfig({ ...valid, VITE_NICE_ISSUER: "https://nice.example.com/" }, ORIGIN)).toMatchObject({
      ok: true,
      config: { issuer: "https://nice.example.com" },
    });
  });

  it("names VITE_REDIRECT_URI when it does not point at this SPA's /callback", () => {
    for (const uri of ["http://localhost:3000/callback", "http://localhost:5180/login", "not a url"]) {
      const result = readConfig({ ...valid, VITE_REDIRECT_URI: uri }, ORIGIN);
      expect(errorsOf(result).map((error) => error.variable)).toEqual(["VITE_REDIRECT_URI"]);
    }
  });

  it("names VITE_POST_LOGOUT_REDIRECT_URI when it is not a URL", () => {
    const result = readConfig({ ...valid, VITE_POST_LOGOUT_REDIRECT_URI: "/home" }, ORIGIN);

    expect(errorsOf(result).map((error) => error.variable)).toEqual(["VITE_POST_LOGOUT_REDIRECT_URI"]);
  });

  it("reports every broken variable at once", () => {
    const result = readConfig({}, ORIGIN);

    expect(errorsOf(result).map((error) => error.variable)).toEqual([
      "VITE_NICE_ISSUER",
      "VITE_NICE_CLIENT_ID",
      "VITE_SERVICES",
    ]);
  });
});

describe("ConfigErrorPage", () => {
  it("shows the name of each broken variable", () => {
    const html = renderToStaticMarkup(<ConfigErrorPage errors={errorsOf(readConfig({ ...valid, VITE_NICE_CLIENT_ID: "", VITE_SERVICES: "{" }, ORIGIN))} />);

    expect(html).toContain("<code>VITE_NICE_CLIENT_ID</code>");
    expect(html).toContain("<code>VITE_SERVICES</code>");
    expect(html).not.toContain("Masuk");
  });
});
