// NICE replaces the scopes of the grant with the ones asked on every refresh,
// and a refresh may only ask for grant scopes plus nice:org/nice:aud/nice:prj.
// Login and every refresh send this same base scope, so userinfo keeps working.
export const BASE_SCOPE = "openid profile email offline_access";

// TokenKey names one token of the session. Without service it is the client
// token (aud = client_id, for userinfo); service "nice" is the delegated NICE
// API token, which has no project. Without org it is the organization the
// refresh token is bound to.
export interface TokenKey {
  org?: string;
  service?: string;
  project?: string;
}

// scopeFor adds nice:org only when the target organization differs from the
// one the refresh token is bound to: NICE keeps the organization on the new
// refresh token, so later refreshes stay there without it.
export function scopeFor(key: TokenKey, sessionOrg: string | undefined): string {
  let scope = BASE_SCOPE;
  if (key.org && key.org !== sessionOrg) scope += ` nice:org:${key.org}`;
  if (key.service) scope += ` nice:aud:${key.service}`;
  if (key.service && key.project) scope += ` nice:prj:${key.project}`;
  return scope;
}
