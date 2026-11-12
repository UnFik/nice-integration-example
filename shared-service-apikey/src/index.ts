import { serve } from "@hono/node-server";
import { buildApp } from "./app.ts";
import { ConfigError, loadConfig, type Config } from "./config.ts";
import { KeyFileError } from "./keys.ts";
import { consoleLogger } from "./log.ts";

const log = consoleLogger();

let config: Config;
try {
  config = loadConfig(process.env);
} catch (err) {
  if (!(err instanceof ConfigError)) throw err;
  process.stderr.write(`pengiriman: ${err.message}\n`);
  process.exit(1);
}

let built: Awaited<ReturnType<typeof buildApp>>;
try {
  built = await buildApp({ serviceCredential: config.serviceCredential, keysFile: config.keysFile, log });
} catch (err) {
  if (!(err instanceof KeyFileError)) throw err;
  process.stderr.write(`pengiriman: ${err.message}\n`);
  process.exit(1);
}

const server = serve({ fetch: built.app.fetch, hostname: config.host, port: config.port }, (info) => {
  log.info("listening", {
    url: `http://${info.family === "IPv6" ? `[${info.address}]` : info.address}:${info.port}`,
    keys_file: config.keysFile,
    active_keys: built.keys.activeCount(),
  });
});

server.on("error", (err: NodeJS.ErrnoException) => {
  log.error("cannot listen", { addr: `${config.host}:${config.port}`, code: err.code });
  process.exit(1);
});

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.once(signal, () => server.close(() => process.exit(0)));
}
