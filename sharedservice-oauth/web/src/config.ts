import { InsecureIssuerError, issuerTransport } from "./oidc";

export interface AppConfig {
  issuer: string;
  clientId: string;
  redirectUri: string;
  postLogoutRedirectUri: string;
  inventarisUrl: string;
}

export interface ConfigError {
  variable: string;
  message: string;
}

export type ConfigResult = { ok: true; config: AppConfig } | { ok: false; errors: ConfigError[] };

export function readConfig(env: Record<string, unknown>, origin: string): ConfigResult {
  const errors: ConfigError[] = [];
  const fail = (variable: string, message: string) => {
    errors.push({ variable, message });
  };
  const value = (name: string) => (typeof env[name] === "string" ? (env[name] as string).trim() : "");

  const issuer = readIssuer(value("VITE_NICE_ISSUER"), fail);
  const clientId = value("VITE_NICE_CLIENT_ID");
  if (!clientId) fail("VITE_NICE_CLIENT_ID", "wajib diisi dengan client_id service client public dari NICE.");
  const redirectUri = readRedirect(value("VITE_REDIRECT_URI") || `${origin}/callback`, origin, fail);
  const postLogoutRedirectUri = readUrl(
    "VITE_POST_LOGOUT_REDIRECT_URI",
    value("VITE_POST_LOGOUT_REDIRECT_URI") || `${origin}/`,
    fail,
  );
  const inventarisUrl = readEndpoint(value("VITE_INVENTARIS_URL"), fail);

  if (errors.length > 0 || !issuer || !redirectUri || !postLogoutRedirectUri || !inventarisUrl) {
    return { ok: false, errors };
  }
  return { ok: true, config: { issuer, clientId, redirectUri, postLogoutRedirectUri, inventarisUrl } };
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

function readEndpoint(raw: string, fail: Fail): string | undefined {
  const name = "VITE_INVENTARIS_URL";
  if (!raw) {
    fail(name, "wajib diisi dengan alamat service inventaris, mis. http://127.0.0.1:8081.");
    return undefined;
  }
  const url = parseUrl(raw);
  if (!url || (url.protocol !== "http:" && url.protocol !== "https:") || url.search || url.hash) {
    fail(name, `"${raw}" harus URL http atau https tanpa query, mis. http://127.0.0.1:8081.`);
    return undefined;
  }
  return url.href.replace(/\/$/, "");
}

function readUrl(name: string, raw: string, fail: Fail): string | undefined {
  const url = parseUrl(raw);
  if (!url || (url.protocol !== "http:" && url.protocol !== "https:")) {
    fail(name, `"${raw}" bukan URL http atau https yang sah.`);
    return undefined;
  }
  return url.href;
}

function parseUrl(raw: string): URL | undefined {
  try {
    return new URL(raw);
  } catch {
    return undefined;
  }
}
