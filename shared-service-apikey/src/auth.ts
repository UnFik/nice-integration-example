import { timingSafeEqual } from "node:crypto";
import type { MiddlewareHandler } from "hono";
import { sha256, type KeyStore } from "./keys.ts";

export type CredentialKind = "service" | "mirrored";

export type AppEnv = {
  Variables: {
    credential: CredentialKind;
    organization: string;
    principal: string | null;
  };
};

export const API_KEY_HEADER = "X-Api-Key";

// The NICE health probe sends no credential, and the catalog import reads the document without one.
const OPEN_PATHS = new Set(["/healthz", "/openapi.json"]);

export function apiKeyAuth(serviceDigest: Buffer, keys: KeyStore): MiddlewareHandler<AppEnv> {
  return async (c, next) => {
    if (OPEN_PATHS.has(c.req.path)) return next();
    const presented = c.req.header(API_KEY_HEADER);
    if (presented) {
      // Hashing first gives equal-length inputs for timingSafeEqual whatever the caller sends.
      const digest = sha256(presented);
      const isService = timingSafeEqual(digest, serviceDigest);
      const mirrored = keys.match(digest);
      if (isService || mirrored) {
        c.set("credential", isService ? "service" : "mirrored");
        return next();
      }
    }
    return c.json({ error: "unauthorized" }, 401);
  };
}

// Mirrored keys only ever call shipments; managing keys needs the service credential that NICE holds.
export const requireServiceCredential: MiddlewareHandler<AppEnv> = async (c, next) => {
  if (c.get("credential") !== "service") {
    return c.json({ error: "admin_credential_required" }, 403);
  }
  return next();
};

// The X-Nice-* headers are trusted only because a valid credential proves the request came through the NICE proxy,
// which drops the values a client sends and sets its own from the verified token.
export const requireOrganization: MiddlewareHandler<AppEnv> = async (c, next) => {
  const organization = c.req.header("X-Nice-Organization")?.trim();
  if (!organization) {
    return c.json({ error: "missing_organization" }, 400);
  }
  c.set("organization", organization);
  c.set("principal", c.req.header("X-Nice-Principal")?.trim() || null);
  return next();
};
