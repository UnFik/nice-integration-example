import { randomBytes } from "node:crypto";
import { createRoute, z, type OpenAPIHono } from "@hono/zod-openapi";
import { requireOrganization, type AppEnv } from "./auth.ts";

export type Shipment = {
  id: string;
  destination: string;
  status: "created" | "cancelled";
  created_by: string | null;
  create_time: string;
};

// In memory on purpose: the example is about the integration, not about storage.
export function createShipmentStore(now: () => Date = () => new Date()) {
  const byOrg = new Map<string, Map<string, Shipment>>();
  const tenant = (org: string) => {
    let m = byOrg.get(org);
    if (!m) byOrg.set(org, (m = new Map()));
    return m;
  };
  return {
    list(org: string): Shipment[] {
      return [...tenant(org).values()];
    },
    create(org: string, destination: string, createdBy: string | null): Shipment {
      const s: Shipment = {
        id: "shp_" + randomBytes(10).toString("hex"),
        destination,
        status: "created",
        created_by: createdBy,
        create_time: now().toISOString(),
      };
      tenant(org).set(s.id, s);
      return s;
    },
    cancel(org: string, id: string): Shipment | "not_found" | "already_cancelled" {
      const s = tenant(org).get(id);
      if (!s) return "not_found";
      if (s.status === "cancelled") return "already_cancelled";
      s.status = "cancelled";
      return s;
    },
  };
}

export type ShipmentStore = ReturnType<typeof createShipmentStore>;

const ShipmentSchema = z
  .object({
    id: z.string().openapi({ example: "shp_4f1c0b9a7e2d5c3b8a61" }),
    destination: z.string().openapi({ example: "Gudang Timur, Surabaya" }),
    status: z.enum(["created", "cancelled"]),
    created_by: z.string().nullable().openapi({ description: "X-Nice-Principal of the creator", example: "sa_01a1" }),
    create_time: z.string().openapi({ format: "date-time" }),
  })
  .openapi("Shipment");

const ShipmentListSchema = z.object({ shipments: z.array(ShipmentSchema) }).openapi("ShipmentList");

const CreateShipmentSchema = z
  .object({ destination: z.string().trim().min(1).max(200).openapi({ example: "Gudang Timur, Surabaya" }) })
  .openapi("CreateShipmentRequest");

const ErrorSchema = z
  .object({ error: z.string(), error_description: z.string().optional() })
  .openapi("Error");

const errorResponse = (description: string) => ({
  description,
  content: { "application/json": { schema: ErrorSchema } },
});

const IdParam = z.object({ id: z.string().openapi({ param: { name: "id", in: "path" }, example: "shp_4f1c0b9a7e2d5c3b8a61" }) });

export const PERMISSIONS = {
  listShipments: "pengiriman.shipments.read",
  createShipment: "pengiriman.shipments.create",
  cancelShipment: "pengiriman.shipments.cancel",
} as const;

export const listShipments = createRoute({
  method: "get",
  path: "/shipments",
  operationId: "listShipments",
  tags: ["Shipments"],
  summary: "List the shipments of the organization",
  "x-nice-permission": PERMISSIONS.listShipments,
  responses: {
    200: { description: "Shipments", content: { "application/json": { schema: ShipmentListSchema } } },
    400: errorResponse("missing_organization"),
  },
});

export const createShipment = createRoute({
  method: "post",
  path: "/shipments",
  operationId: "createShipment",
  tags: ["Shipments"],
  summary: "Create a shipment",
  "x-nice-permission": PERMISSIONS.createShipment,
  request: {
    body: { required: true, content: { "application/json": { schema: CreateShipmentSchema } } },
  },
  responses: {
    201: { description: "Created", content: { "application/json": { schema: ShipmentSchema } } },
    400: errorResponse("invalid_argument or missing_organization"),
  },
});

export const cancelShipment = createRoute({
  method: "post",
  path: "/shipments/{id}/cancel",
  operationId: "cancelShipment",
  tags: ["Shipments"],
  summary: "Cancel a shipment",
  "x-nice-permission": PERMISSIONS.cancelShipment,
  request: { params: IdParam },
  responses: {
    200: { description: "Cancelled", content: { "application/json": { schema: ShipmentSchema } } },
    400: errorResponse("missing_organization"),
    404: errorResponse("not_found"),
    409: errorResponse("already_cancelled"),
  },
});

export function registerShipments(app: OpenAPIHono<AppEnv>, store: ShipmentStore): void {
  app.use("/shipments", requireOrganization);
  app.use("/shipments/*", requireOrganization);

  app.openapi(listShipments, (c) => c.json({ shipments: store.list(c.get("organization")) }, 200));

  app.openapi(createShipment, (c) => {
    const { destination } = c.req.valid("json");
    return c.json(store.create(c.get("organization"), destination, c.get("principal")), 201);
  });

  app.openapi(cancelShipment, (c) => {
    const result = store.cancel(c.get("organization"), c.req.valid("param").id);
    if (result === "not_found") return c.json({ error: "not_found" }, 404);
    if (result === "already_cancelled") return c.json({ error: "already_cancelled" }, 409);
    return c.json(result, 200);
  });
}
