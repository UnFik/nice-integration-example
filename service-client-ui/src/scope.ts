// NICE replaces the scopes of the grant with the ones asked on every refresh,
// and a refresh may only ask for grant scopes plus nice:aud/nice:prj. Login and
// every refresh send this same base scope, so userinfo keeps working.
export const BASE_SCOPE = "openid profile email offline_access";

export interface ServiceKey {
  slug: string;
  project?: string;
}

export const scopeOf = (service: ServiceKey): string =>
  `${BASE_SCOPE} nice:aud:${service.slug}` + (service.project ? ` nice:prj:${service.project}` : "");
