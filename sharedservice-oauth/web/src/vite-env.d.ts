/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_NICE_ISSUER?: string;
  readonly VITE_NICE_CLIENT_ID?: string;
  readonly VITE_REDIRECT_URI?: string;
  readonly VITE_POST_LOGOUT_REDIRECT_URI?: string;
  readonly VITE_INVENTARIS_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
