export type Claims = Record<string, unknown>;

// Decodes the payload only. Nothing is verified: the SPA shows the claims and
// schedules renewals from iat/exp, while the services verify the tokens.
export function decodeClaims(token: string): Claims | undefined {
  const parts = token.split(".");
  if (parts.length !== 3) return undefined;
  try {
    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const binary = atob(base64.padEnd(Math.ceil(base64.length / 4) * 4, "="));
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
    const value: unknown = JSON.parse(new TextDecoder().decode(bytes));
    return value !== null && typeof value === "object" && !Array.isArray(value) ? (value as Claims) : undefined;
  } catch {
    return undefined;
  }
}

export function tokenTimes(token: string): { issuedAt: number; expiresAt: number } | undefined {
  const claims = decodeClaims(token);
  if (typeof claims?.iat !== "number" || typeof claims.exp !== "number") return undefined;
  return { issuedAt: claims.iat * 1000, expiresAt: claims.exp * 1000 };
}
