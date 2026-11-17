import { describe, expect, it } from "vitest";
import { explainCallError, explainResponse, explainTokenError, parseErrorBody } from "./explain";

describe("explain", () => {
  it("reads error and error_description from a JSON body and tolerates anything else", () => {
    expect(parseErrorBody('{"error":"x","error_description":"token_stale"}')).toEqual({ code: "x", description: "token_stale" });
    expect(parseErrorBody("<html>")).toEqual({});
    expect(parseErrorBody("null")).toEqual({});
  });

  it("explains only failures and names the known causes", () => {
    expect(explainResponse(200, {})).toBeUndefined();
    expect(explainResponse(401, { description: "token_stale" })?.title).toBe("Token basi");
    expect(explainResponse(403, {})?.title).toBe("Izin tidak cukup");
    expect(explainResponse(400, { code: "permission_denied" })?.title).toBe("Izin tidak cukup");
    expect(explainResponse(401, {})?.title).toBe("Token ditolak");
    expect(explainResponse(502, {})?.title).toBe("Service gagal memproses");
  });

  it("explains token and network failures", () => {
    expect(explainTokenError("invalid_scope", false).hint).toContain("belum aktif di project");
    expect(explainTokenError(undefined, true).title).toBe("NICE belum menjawab");
    expect(explainCallError().hint).toContain("CORS");
  });
});
