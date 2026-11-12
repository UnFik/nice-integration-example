export type Config = {
  serviceCredential: string;
  host: string;
  port: number;
  keysFile: string;
};

export class ConfigError extends Error {}

export const DEFAULT_LISTEN_ADDR = "127.0.0.1:8082";
export const DEFAULT_KEYS_FILE = "data/keys.json";

export function loadConfig(env: Record<string, string | undefined>): Config {
  const serviceCredential = env.SERVICE_CREDENTIAL?.trim() ?? "";
  if (serviceCredential === "") {
    throw new ConfigError(
      "SERVICE_CREDENTIAL is required: the credential NICE sends in X-Api-Key (see .env.example)",
    );
  }
  const { host, port } = parseListenAddr(env.LISTEN_ADDR?.trim() || DEFAULT_LISTEN_ADDR);
  const keysFile = env.KEYS_FILE?.trim() || DEFAULT_KEYS_FILE;
  return { serviceCredential, host, port, keysFile };
}

function parseListenAddr(value: string): { host: string; port: number } {
  const colon = value.lastIndexOf(":");
  const host = value.slice(0, colon).replace(/^\[(.*)\]$/, "$1");
  const portText = value.slice(colon + 1);
  const port = Number(portText);
  if (colon <= 0 || host === "" || !/^\d+$/.test(portText) || port < 1 || port > 65535) {
    throw new ConfigError(`LISTEN_ADDR must be host:port, for example ${DEFAULT_LISTEN_ADDR}`);
  }
  return { host, port };
}
