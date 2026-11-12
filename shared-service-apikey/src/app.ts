import { OpenAPIHono, z } from "@hono/zod-openapi";
import { HTTPException } from "hono/http-exception";
import { apiKeyAuth, requireServiceCredential, type AppEnv } from "./auth.ts";
import { openKeyStore, sha256, type KeyStore } from "./keys.ts";
import type { Logger } from "./log.ts";
import { createShipmentStore, registerShipments } from "./shipments.ts";

export type AppDeps = {
  serviceCredential: string;
  keys: KeyStore;
  log: Logger;
};

export const KEY_ROUTES = {
  generate: { method: "POST", path: "/keys" },
  rotate: { method: "POST", path: "/keys/{key_id}/rotate" },
  revoke: { method: "DELETE", path: "/keys/{key_id}" },
} as const;

const OPENAPI_INFO = {
  openapi: "3.0.3",
  info: {
    title: "Pengiriman",
    version: "1.0.0",
    description: "NICE example api_key service. Called through the NICE proxy at /pengiriman/...",
  },
  tags: [{ name: "Shipments", description: "Create, list and cancel shipments" }],
};

const honoPath = (path: string) => path.replace(/\{(\w+)\}/g, ":$1");

const LabelBody = z.object({ label: z.string().trim().min(1).max(200) });

const invalid = (field: string) => ({ error: "invalid_argument", error_description: field });

export function createApp({ serviceCredential, keys, log }: AppDeps): OpenAPIHono<AppEnv> {
  const app = new OpenAPIHono<AppEnv>({
    defaultHook: (result, c) => {
      if (!result.success) {
        const field = result.error.issues[0]?.path.join(".") || "body";
        return c.json(invalid(field), 400);
      }
    },
  });

  app.use(async (c, next) => {
    const start = performance.now();
    await next();
    log.info("request", {
      method: c.req.method,
      path: c.req.path,
      status: c.res.status,
      ms: Math.round(performance.now() - start),
      credential: c.get("credential"),
    });
  });

  app.use(apiKeyAuth(sha256(serviceCredential), keys));

  app.get("/healthz", (c) => c.json({ status: "ok" }));

  // Plain routes stay out of the document, so the catalog never learns the key operations or these two paths.
  app.get("/openapi.json", (c) => c.json(app.getOpenAPIDocument(OPENAPI_INFO)));

  registerShipments(app, createShipmentStore());

  app.use("/keys", requireServiceCredential);
  app.use("/keys/*", requireServiceCredential);

  app.on(KEY_ROUTES.generate.method, honoPath(KEY_ROUTES.generate.path), async (c) => {
    let body: unknown;
    try {
      body = await c.req.json();
    } catch {
      return c.json(invalid("body"), 400);
    }
    const parsed = LabelBody.safeParse(body);
    if (!parsed.success) return c.json(invalid("label"), 400);
    const issued = await keys.generate(parsed.data.label);
    log.info("key generated", { key_id: issued.key_id, label: parsed.data.label });
    return c.json(issued, 201);
  });

  // NICE also sends {"key_id"} in the body; the path already names the key.
  app.on(KEY_ROUTES.rotate.method, honoPath(KEY_ROUTES.rotate.path), async (c) => {
    const issued = await keys.rotate(c.req.param("key_id") ?? "");
    if (!issued) return c.json({ error: "not_found" }, 404);
    log.info("key rotated", { key_id: issued.key_id });
    return c.json(issued, 200);
  });

  app.on(KEY_ROUTES.revoke.method, honoPath(KEY_ROUTES.revoke.path), async (c) => {
    const keyId = c.req.param("key_id") ?? "";
    if (!(await keys.revoke(keyId))) return c.json({ error: "not_found" }, 404);
    log.info("key revoked", { key_id: keyId });
    return c.body(null, 204);
  });

  app.notFound((c) => c.json({ error: "not_found" }, 404));

  app.onError((err, c) => {
    if (err instanceof HTTPException && err.status === 400) {
      return c.json(invalid("body"), 400);
    }
    if (err instanceof HTTPException && err.status === 415) {
      return c.json({ error: "unsupported_media_type", error_description: "Content-Type must be application/json" }, 415);
    }
    log.error("unhandled error", { path: c.req.path, error: err.name });
    return c.json({ error: "internal" }, 500);
  });

  return app;
}

export async function buildApp(opts: { serviceCredential: string; keysFile: string; log: Logger }) {
  const keys = await openKeyStore(opts.keysFile);
  return { app: createApp({ serviceCredential: opts.serviceCredential, keys, log: opts.log }), keys };
}
