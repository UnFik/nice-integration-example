import type { TokenManager } from "./token-manager";

export interface ContextProject {
  id: string;
  name: string;
  slug: string;
  services: string[];
}

export interface ContextOrganization {
  id: string;
  name: string;
  slug: string;
  projects: ContextProject[];
  projectsTruncated: boolean;
}

export interface Contexts {
  currentOrganizationId: string;
  organizations: ContextOrganization[];
}

export class ContextsError extends Error {
  constructor(
    message: string,
    readonly status?: number,
    readonly code?: string,
  ) {
    super(message);
    this.name = "ContextsError";
  }
}

// The largest page NICE allows, so a user with few organizations needs one call.
const PAGE_SIZE = 50;

// loadContexts reads GET /api/v1/me/contexts with the delegated aud=nice token
// of org and follows every page. The delegated token names the service through
// its nice_service claim, so the request must not send `service`.
export async function loadContexts(tokens: TokenManager, issuer: string, org: string | undefined): Promise<Contexts> {
  const organizations: ContextOrganization[] = [];
  let current = "";
  let pageToken = "";
  do {
    const url = new URL(`${issuer}/api/v1/me/contexts`);
    url.searchParams.set("page_size", String(PAGE_SIZE));
    if (pageToken) url.searchParams.set("page_token", pageToken);
    const response = await tokens.call({ org, service: "nice" }, url.href, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    const body = (await response.json().catch(() => undefined)) as Record<string, unknown> | undefined;
    if (!response.ok) {
      const code = typeof body?.error_code === "string" ? body.error_code : `http_${response.status}`;
      const message = typeof body?.message === "string" ? ` (${body.message})` : "";
      throw new ContextsError(`NICE menolak GET /api/v1/me/contexts: ${code}${message}.`, response.status, code);
    }
    if (!body || !Array.isArray(body.organizations)) {
      throw new ContextsError("Jawaban GET /api/v1/me/contexts tidak memuat organizations.");
    }
    current = text(body.current_organization_id) || current;
    organizations.push(...body.organizations.map(organization));
    pageToken = text((body.page as Record<string, unknown> | undefined)?.next_page_token);
  } while (pageToken);
  return { currentOrganizationId: current, organizations };
}

function organization(raw: unknown): ContextOrganization {
  const value = (raw ?? {}) as Record<string, unknown>;
  const projects = Array.isArray(value.projects) ? value.projects : [];
  return {
    id: text(value.id),
    name: text(value.name),
    slug: text(value.slug),
    projects: projects.map((item: unknown) => {
      const project = (item ?? {}) as Record<string, unknown>;
      return {
        id: text(project.id),
        name: text(project.name),
        slug: text(project.slug),
        services: Array.isArray(project.services) ? project.services.filter((s) => typeof s === "string") : [],
      };
    }),
    projectsTruncated: value.projects_truncated === true,
  };
}

function text(value: unknown): string {
  return typeof value === "string" ? value : "";
}
