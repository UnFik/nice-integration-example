import type { ActiveContext } from "./contexts";
import { type KeyValueStorage, PREFIX } from "./storage";

// Only org and project ids: no token, and clearSession removes the key with the rest of the session.
export const RECENT_KEY = `${PREFIX}recent_contexts`;
const LIMIT = 5;

export function readRecent(storage: KeyValueStorage): ActiveContext[] {
  try {
    const raw: unknown = JSON.parse(storage.getItem(RECENT_KEY) ?? "[]");
    if (!Array.isArray(raw)) return [];
    return raw
      .filter((entry): entry is ActiveContext => typeof entry?.org === "string" && typeof entry?.project === "string")
      .map((entry) => ({ org: entry.org, project: entry.project }))
      .slice(0, LIMIT);
  } catch {
    return [];
  }
}

// remember puts the context first and drops older duplicates.
export function remember(storage: KeyValueStorage, context: Required<ActiveContext>): ActiveContext[] {
  const next = [context, ...readRecent(storage).filter((entry) => entry.org !== context.org || entry.project !== context.project)].slice(0, LIMIT);
  try {
    storage.setItem(RECENT_KEY, JSON.stringify(next));
  } catch {
    // A full or blocked sessionStorage only loses the shortcut list.
  }
  return next;
}

export type ManualContextError = { field: "org" | "project"; message: string };

export function validateManualContext(org: string, project: string): { org: string; project: string } | ManualContextError {
  const orgId = org.trim();
  const projectId = project.trim();
  if (!orgId || /\s/.test(orgId)) return { field: "org", message: "ID organisasi wajib diisi, tanpa spasi." };
  if (!projectId.startsWith("prj_") || projectId.length <= 4 || /\s/.test(projectId)) {
    return { field: "project", message: "ID project diawali prj_, tanpa spasi." };
  }
  return { org: orgId, project: projectId };
}
