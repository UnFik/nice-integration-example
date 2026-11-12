import { InsecureIssuerError, issuerTransport } from "./oidc";

export interface ServiceConfig {
  slug: string;
  endpoint: string;
  path: string;
  project?: string;
}

export interface AppConfig {
  issuer: string;
  clientId: string;
  redirectUri: string;
  postLogoutRedirectUri: string;
  services: ServiceConfig[];
}

export interface ConfigError {
  variable: string;
  message: string;
}

export type ConfigResult = { ok: true; config: AppConfig } | { ok: false; errors: ConfigError[] };

const SLUG = /^[a-z][a-z0-9_]{1,31}$/;
const PROJECT = /^prj_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const SERVICE_FIELDS = new Set(["slug", "endpoint", "path", "project"]);

export function readConfig(env: Record<string, unknown>, origin: string): ConfigResult {
  const errors: ConfigError[] = [];
  const fail = (variable: string, message: string) => {
    errors.push({ variable, message });
  };
  const value = (name: string) => (typeof env[name] === "string" ? (env[name] as string).trim() : "");

  const issuer = readIssuer(value("VITE_NICE_ISSUER"), fail);
  const clientId = value("VITE_NICE_CLIENT_ID");
  if (!clientId) fail("VITE_NICE_CLIENT_ID", "wajib diisi dengan client_id OAuth client public dari NICE.");
  const redirectUri = readRedirect(value("VITE_REDIRECT_URI") || `${origin}/callback`, origin, fail);
  const postLogoutRedirectUri = readUrl(
    "VITE_POST_LOGOUT_REDIRECT_URI",
    value("VITE_POST_LOGOUT_REDIRECT_URI") || `${origin}/`,
    fail,
  );
  const services = readServices(value("VITE_SERVICES"), fail);

  if (errors.length > 0 || !issuer || !redirectUri || !postLogoutRedirectUri || !services) return { ok: false, errors };
  return { ok: true, config: { issuer, clientId, redirectUri, postLogoutRedirectUri, services } };
}

type Fail = (variable: string, message: string) => void;

function readIssuer(raw: string, fail: Fail): string | undefined {
  const name = "VITE_NICE_ISSUER";
  if (!raw) {
    fail(name, "wajib diisi dengan alamat NICE, mis. http://localhost:8080.");
    return undefined;
  }
  const url = parseUrl(raw);
  if (!url || (url.protocol !== "http:" && url.protocol !== "https:")) {
    fail(name, `"${raw}" bukan URL http atau https yang sah.`);
    return undefined;
  }
  if (url.search || url.hash || url.username || url.password) {
    fail(name, "tidak boleh memuat query, fragmen, atau kredensial.");
    return undefined;
  }
  try {
    issuerTransport(url);
  } catch (error) {
    if (error instanceof InsecureIssuerError) {
      fail(name, "harus memakai https; http hanya diizinkan untuk localhost, 127.0.0.1, atau [::1].");
      return undefined;
    }
    throw error;
  }
  return url.href.replace(/\/$/, "");
}

function readRedirect(raw: string, origin: string, fail: Fail): string | undefined {
  const url = readUrl("VITE_REDIRECT_URI", raw, fail);
  if (!url) return undefined;
  const parsed = new URL(url);
  if (parsed.origin !== origin || parsed.pathname !== "/callback") {
    fail("VITE_REDIRECT_URI", `harus menunjuk ke ${origin}/callback, karena halaman itu yang menyelesaikan login.`);
    return undefined;
  }
  return url;
}

function readUrl(name: string, raw: string, fail: Fail): string | undefined {
  const url = parseUrl(raw);
  if (!url || (url.protocol !== "http:" && url.protocol !== "https:")) {
    fail(name, `"${raw}" bukan URL http atau https yang sah.`);
    return undefined;
  }
  return url.href;
}

function readServices(raw: string, fail: Fail): ServiceConfig[] | undefined {
  const name = "VITE_SERVICES";
  if (!raw) {
    fail(name, 'wajib diisi dengan array JSON, mis. [{"slug":"inventaris","endpoint":"http://127.0.0.1:8081","path":"/items"}].');
    return undefined;
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    fail(name, `bukan JSON yang sah (${error instanceof Error ? error.message : String(error)}).`);
    return undefined;
  }
  if (!Array.isArray(parsed)) {
    fail(name, "harus berupa array JSON berisi objek {slug, endpoint, path, project?}.");
    return undefined;
  }
  const services: ServiceConfig[] = [];
  let valid = true;
  parsed.forEach((item: unknown, index) => {
    const at = `entri ${index}`;
    if (item === null || typeof item !== "object" || Array.isArray(item)) {
      fail(name, `${at} harus berupa objek {slug, endpoint, path, project?}.`);
      valid = false;
      return;
    }
    const entry = item as Record<string, unknown>;
    const problems: string[] = [];
    const unknown = Object.keys(entry).filter((field) => !SERVICE_FIELDS.has(field));
    if (unknown.length > 0) problems.push(`field tidak dikenal: ${unknown.join(", ")}`);
    if (typeof entry.slug !== "string" || !SLUG.test(entry.slug)) {
      problems.push("slug harus slug SharedService NICE (huruf kecil, angka, garis bawah; 2-32 karakter)");
    }
    const endpoint = typeof entry.endpoint === "string" ? parseUrl(entry.endpoint) : undefined;
    if (!endpoint || (endpoint.protocol !== "http:" && endpoint.protocol !== "https:") || endpoint.search || endpoint.hash) {
      problems.push("endpoint harus URL http atau https tanpa query, mis. http://127.0.0.1:8081");
    }
    if (typeof entry.path !== "string" || !entry.path.startsWith("/")) problems.push('path harus diawali "/"');
    if (entry.project !== undefined && (typeof entry.project !== "string" || !PROJECT.test(entry.project))) {
      problems.push("project, bila diisi, harus ID project NICE (prj_<uuid>)");
    }
    if (problems.length > 0 || !endpoint) {
      fail(name, `${at}: ${problems.join("; ")}.`);
      valid = false;
      return;
    }
    services.push({
      slug: entry.slug as string,
      endpoint: endpoint.href.replace(/\/$/, ""),
      path: entry.path as string,
      ...(entry.project ? { project: entry.project as string } : {}),
    });
  });
  return valid ? services : undefined;
}

function parseUrl(raw: string): URL | undefined {
  try {
    return new URL(raw);
  } catch {
    return undefined;
  }
}
