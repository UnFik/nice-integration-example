import type { TokenView } from "../token-manager";

export type TokenPhase = "active" | "soon" | "expired";

// The manager renews at refreshAt, and never later than a minute before exp.
const SAFETY_MS = 60_000;

export const PHASE_LABEL: Record<TokenPhase, string> = {
  active: "Aktif",
  soon: "Segera diperbarui",
  expired: "Kedaluwarsa",
};

export function tokenPhase(token: Pick<TokenView, "refreshAt" | "expiresAt">, now: number): TokenPhase {
  if (now >= token.expiresAt) return "expired";
  if (now >= Math.min(token.refreshAt, token.expiresAt - SAFETY_MS)) return "soon";
  return "active";
}

export function remainingMs(token: Pick<TokenView, "expiresAt">, now: number): number {
  return Math.max(0, token.expiresAt - now);
}

// ageRatio is how much of the token's life has passed, from 0 to 1.
export function ageRatio(token: Pick<TokenView, "issuedAt" | "expiresAt">, now: number): number {
  const life = token.expiresAt - token.issuedAt;
  if (life <= 0) return 1;
  return Math.min(1, Math.max(0, (now - token.issuedAt) / life));
}
