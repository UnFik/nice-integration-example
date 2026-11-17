import { describe, expect, it } from "vitest";
import { ApiError, createApi, loginUrl, toItem } from "./api";
import { json } from "./test/fake-backend";

function client(answer: (url: string, init: RequestInit) => Response, csrf = "csrf-1") {
  const calls: { url: string; init: RequestInit }[] = [];
  const fetcher = async (input: RequestInfo | URL, init: RequestInit = {}) => {
    calls.push({ url: String(input), init });
    return answer(String(input), init);
  };
  return { api: createApi(fetcher, () => csrf), calls };
}

const header = (init: RequestInit, name: string) => new Headers(init.headers).get(name);

const stored = { id: "itm_1", name: "Kabel", quantity: 3, created_by: "usr_1", acted_by: "agt_1", create_time: "2026-11-01T10:00:00Z", update_time: "2026-11-02T10:00:00Z" };

describe("createApi", () => {
  it("reads the session with a relative URL and same-origin credentials, without the CSRF header", async () => {
    const { api, calls } = client(() =>
      json(200, { user: { id: "usr_1", name: "Siti", email: "s@x" }, context: { org: "org_a", project: "prj_p" }, perms: ["a", 1], csrf_token: "t", context_error: "" }),
    );

    const info = await api.session();

    expect(info).toEqual({ user: { id: "usr_1", name: "Siti", email: "s@x" }, context: { org: "org_a", project: "prj_p" }, perms: ["a"], csrfToken: "t", contextError: undefined });
    expect(calls[0].url).toBe("/auth/session");
    expect(calls[0].init).toMatchObject({ method: "GET", credentials: "same-origin" });
    expect(header(calls[0].init, "X-CSRF-Token")).toBeNull();
  });

  it("reads a session without context and with a context error", async () => {
    const { api } = client(() => json(200, { user: {}, context: null, perms: [], csrf_token: "t", context_error: "access_denied" }));

    await expect(api.session()).resolves.toMatchObject({ context: undefined, contextError: "access_denied" });
  });

  it("sends the CSRF token and a JSON body on every write", async () => {
    const { api, calls } = client((url) => (url === "/auth/logout" ? new Response(null, { status: 204 }) : json(200, stored)));

    await api.createItem({ name: "Kabel", quantity: 3 });
    await api.updateItem("itm/1", { name: "Kabel", quantity: 4 });
    await api.removeItem("itm_1");
    await api.logout();

    expect(calls.map((call) => [call.init.method, call.url])).toEqual([
      ["POST", "/items"],
      ["PUT", "/items/itm%2F1"],
      ["DELETE", "/items/itm_1"],
      ["POST", "/auth/logout"],
    ]);
    for (const call of calls) expect(header(call.init, "X-CSRF-Token")).toBe("csrf-1");
    expect(calls[0].init.body).toBe('{"name":"Kabel","quantity":3}');
    expect(header(calls[0].init, "Content-Type")).toBe("application/json");
    expect(header(calls[3].init, "Content-Type")).toBeNull();
  });

  it("posts the chosen context and reads the new session", async () => {
    const { api, calls } = client(() => json(200, { user: {}, context: { org: "org_b", project: "prj_r" }, perms: [], csrf_token: "t" }));

    await expect(api.switchContext({ org: "org_b", project: "prj_r" })).resolves.toMatchObject({ context: { org: "org_b", project: "prj_r" } });
    expect(calls[0].init.body).toBe('{"org":"org_b","project":"prj_r"}');
  });

  it("maps the contexts answer", async () => {
    const { api } = client(() =>
      json(200, {
        current_organization_id: "org_a",
        organizations: [{ id: "org_a", name: "A", slug: "a", projects: [{ id: "prj_p", name: "P", slug: "p" }], projects_truncated: true }],
      }),
    );

    await expect(api.contexts()).resolves.toEqual({
      currentOrganizationId: "org_a",
      organizations: [{ id: "org_a", name: "A", slug: "a", projects: [{ id: "prj_p", name: "P", slug: "p" }], projectsTruncated: true }],
    });
  });

  it("raises the error and description of the backend", async () => {
    const { api } = client(() => json(409, { error: "context_refused", error_description: "invalid_scope" }));

    await expect(api.switchContext({ org: "o", project: "p" })).rejects.toMatchObject({ status: 409, code: "context_refused", description: "invalid_scope" });
  });

  it("rejects answers without the fields the UI needs", async () => {
    await expect(client(() => json(200, {})).api.listItems()).rejects.toBeInstanceOf(ApiError);
    await expect(client(() => json(200, { user: {} })).api.session()).rejects.toBeInstanceOf(ApiError);
    await expect(client(() => json(200, {})).api.contexts()).rejects.toBeInstanceOf(ApiError);
  });
});

describe("loginUrl and toItem", () => {
  it("navigates to the backend login, with prompt=select_account for another account", () => {
    expect(loginUrl()).toBe("/auth/login");
    expect(loginUrl({ selectAccount: true })).toBe("/auth/login?prompt=select_account");
  });

  it("maps an item and fills what an answer leaves out", () => {
    expect(toItem(stored)).toEqual({ id: "itm_1", name: "Kabel", quantity: 3, createdBy: "usr_1", actedBy: "agt_1", createTime: "2026-11-01T10:00:00Z", updateTime: "2026-11-02T10:00:00Z" });
    expect(toItem({ id: "itm_1" })).toEqual({ id: "itm_1", name: "", quantity: 0, createdBy: "", actedBy: undefined, createTime: undefined, updateTime: undefined });
  });
});
