export const PREFIX = "nice-example.";

export const keys = {
  refreshToken: `${PREFIX}refresh_token`,
  idToken: `${PREFIX}id_token`,
  login: `${PREFIX}login`,
} as const;

export type KeyValueStorage = Pick<Storage, "getItem" | "setItem" | "removeItem" | "key" | "length">;

export function clearSession(storage: KeyValueStorage): void {
  const owned: string[] = [];
  for (let i = 0; i < storage.length; i++) {
    const key = storage.key(i);
    if (key?.startsWith(PREFIX)) owned.push(key);
  }
  for (const key of owned) storage.removeItem(key);
}
